//go:build windows && amd64

package sysprobe

import (
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// The Windows implementation is stdlib-only: psapi/kernel32 for memory, a
// small DXGI COM walk for VRAM, and PDH for GPU utilization. Every probe
// fails soft — an HRESULT or a missing counter turns into an unset OK flag,
// never an error the host has to react to.

// proberInterval is the minimum time between two real samples. It lives here
// rather than in the shared file because it is this implementation's choice:
// a port that reads a cheaper source is free to pick its own.
const proberInterval = time.Second

var (
	psapi                 = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemory  = psapi.NewProc("GetProcessMemoryInfo")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGlobalMemory      = kernel32.NewProc("GlobalMemoryStatusEx")
	procGetCurrentProcess = kernel32.NewProc("GetCurrentProcess")
	dxgi                  = syscall.NewLazyDLL("dxgi.dll")
	procCreateDXGIFactory = dxgi.NewProc("CreateDXGIFactory1")
	pdh                   = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery      = pdh.NewProc("PdhOpenQueryW")
	procPdhAddCounter     = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollect        = pdh.NewProc("PdhCollectQueryData")
	procPdhFormatArray    = pdh.NewProc("PdhGetFormattedCounterArrayW")
	procPdhCloseQuery     = pdh.NewProc("PdhCloseQuery")
)

// Prober samples process and system metrics for the dev panel. Create it
// once per process; the PDH query it opens lives as long as the process
// (the OS reclaims it at exit).
type Prober struct {
	pid            uint32
	vendor, device uint32

	pdhQuery   uintptr
	pdhCounter uintptr
	pdhOK      bool

	// factory is the DXGI factory the VRAM walk enumerates from. It is kept
	// across samples: adapters do not come and go, and creating one per
	// sample means a COM object built and torn down every second for a
	// reading that never changes shape. nil until the first successful
	// creation, and left nil when DXGI is unavailable.
	factory unsafe.Pointer

	last   time.Time
	cached Sample
}

// NewProber opens the probes that need a handle. vendor/device identify the
// render adapter (from the graphics API's adapter info) so the VRAM reading
// lands on the same GPU the game renders to; zeros fall back to the first
// real adapter.
func NewProber(vendor, device uint32) *Prober {
	p := &Prober{pid: uint32(syscall.Getpid()), vendor: vendor, device: device}

	var query uintptr
	if status, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&query))); status == 0 && query != 0 {
		path, err := syscall.UTF16PtrFromString(`\GPU Engine(*)\Utilization Percentage`)
		var counter uintptr
		if err == nil {
			status, _, _ := procPdhAddCounter.Call(query,
				uintptr(unsafe.Pointer(path)), 0, uintptr(unsafe.Pointer(&counter)))
			if status == 0 && counter != 0 {
				p.pdhQuery = query
				p.pdhCounter = counter
				p.pdhOK = true
				// Rate counters read garbage until the second collect; the
				// first real sample happens a second later, so prime now.
				_, _, _ = procPdhCollect.Call(query)
			}
		}
		if !p.pdhOK {
			// The counter did not attach — a machine with no GPU engine
			// counters, or a PDH that refused the path. The query is of no
			// further use, and holding it open would leak a kernel handle
			// for the life of the process.
			_, _, _ = procPdhCloseQuery.Call(query)
		}
	}
	return p
}

// Close releases the probe's OS handles. The host keeps one Prober for the
// life of the process and the OS reclaims everything at exit, so calling it
// is optional — it exists so a test can open and drop probes without
// accumulating handles.
func (p *Prober) Close() {
	if p.pdhOK {
		_, _, _ = procPdhCloseQuery.Call(p.pdhQuery)
		p.pdhQuery, p.pdhCounter, p.pdhOK = 0, 0, false
	}
	if p.factory != nil {
		comRelease(p.factory)
		p.factory = nil
	}
}

// Sample returns the latest readings, querying the OS at most once per
// proberInterval; frames in between get the cached sample.
func (p *Prober) Sample(now time.Time) Sample {
	if now.Sub(p.last) < proberInterval {
		return p.cached
	}
	p.last = now

	var s Sample
	s.MemProcessBytes, s.MemSystemUsedBytes, s.MemSystemTotalBytes, s.MemOK = memoryUsage()
	s.VRAMProcessBytes, s.VRAMBudgetBytes, s.VRAMOK = p.videoMemory()
	s.GPUProcessPercent, s.GPUSystemPercent, s.GPUOK = p.gpuUsage()
	p.cached = s
	return s
}

// --- memory (psapi + kernel32) ---

type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

func memoryUsage() (process, systemUsed, systemTotal uint64, ok bool) {
	handle, _, _ := procGetCurrentProcess.Call()
	counters := processMemoryCounters{cb: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	if ret, _, _ := procGetProcessMemory.Call(handle,
		uintptr(unsafe.Pointer(&counters)), uintptr(counters.cb)); ret == 0 {
		return 0, 0, 0, false
	}

	status := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	if ret, _, _ := procGlobalMemory.Call(uintptr(unsafe.Pointer(&status))); ret == 0 {
		return 0, 0, 0, false
	}
	return uint64(counters.workingSetSize), status.totalPhys - status.availPhys, status.totalPhys, true
}

// --- VRAM (DXGI COM) ---

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

var (
	// The factory must be an IDXGIFactory4 even though only EnumAdapters1 is
	// called on it: the adapter's supported interfaces are bounded by the
	// factory version that created it, and IDXGIAdapter3 (QueryVideoMemoryInfo)
	// only exists on adapters born of a factory 4.
	iidIDXGIFactory4 = guid{0x1bc6ea02, 0xef36, 0x464f, [8]byte{0xbf, 0x0c, 0x21, 0xca, 0x39, 0xe5, 0x16, 0x8a}}
	iidIDXGIAdapter3 = guid{0x645967a4, 0x1392, 0x4310, [8]byte{0xa7, 0x98, 0x80, 0x53, 0xce, 0x3e, 0x93, 0xfd}}
)

// COM vtable slots used below. IUnknown is slots 0-2 (QueryInterface,
// AddRef, Release) for every interface; the rest follow the dxgi headers.
const (
	slotQueryInterface = 0
	slotRelease        = 2
	slotEnumAdapters1  = 12 // IDXGIFactory1
	slotGetDesc1       = 10 // IDXGIAdapter1
	// IDXGIAdapter3, after IDXGIAdapter2's GetDesc2 at slot 11 and the two
	// content-protection slots.
	slotQueryVideoMemoryInfo = 14
)

type dxgiAdapterDesc1 struct {
	description           [128]uint16
	vendorID              uint32
	deviceID              uint32
	subSysID              uint32
	revision              uint32
	dedicatedVideoMemory  uintptr
	dedicatedSystemMemory uintptr
	sharedSystemMemory    uintptr
	adapterLuid           uint64
	flags                 uint32
	_                     uint32
}

type dxgiQueryVideoMemoryInfo struct {
	budget                  uint64
	currentUsage            uint64
	availableForReservation uint64
	currentReservation      uint64
}

// comCall invokes vtable slot index on a COM object. Objects travel as
// unsafe.Pointer end to end, so no uintptr round-trip ever stands between
// the GC and a live object. The return is the HRESULT; the error return of
// SyscallN is GetLastError, which COM does not use, so it is dropped.
func comCall(obj unsafe.Pointer, index uintptr, args ...uintptr) uintptr {
	vtable := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtable, index*8))
	ret, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return ret
}

func comRelease(obj unsafe.Pointer) {
	if obj != nil {
		comCall(obj, slotRelease)
	}
}

// videoMemory walks DXGI to the adapter the game renders on and asks the
// kernel's video memory manager for this process's usage there — API
// agnostic, so it counts D3D12 and Vulkan allocations alike.
func (p *Prober) videoMemory() (process, budget uint64, ok bool) {
	factory := p.dxgiFactory()
	if factory == nil {
		return 0, 0, false
	}

	for index := uintptr(0); ; index++ {
		var adapter unsafe.Pointer
		if hr := comCall(factory, slotEnumAdapters1, index, uintptr(unsafe.Pointer(&adapter))); hr != 0 {
			// DXGI_ERROR_NOT_FOUND: no adapter matched. A cached factory
			// never sees an adapter added after it was created, so drop it
			// and let the next sample walk a fresh one.
			p.factory = nil
			comRelease(factory)
			return 0, 0, false
		}
		matched, usage, budget := p.adapterVideoMemory(adapter)
		comRelease(adapter)
		if matched {
			return usage, budget, true
		}
	}
}

// dxgiFactory returns the cached factory, creating it on first use. A nil
// return means DXGI is unavailable, and every later sample retries — the
// probe is allowed to fail, never to break the frame.
func (p *Prober) dxgiFactory() unsafe.Pointer {
	if p.factory != nil {
		return p.factory
	}
	var factory unsafe.Pointer
	if hr, _, _ := procCreateDXGIFactory.Call(
		uintptr(unsafe.Pointer(&iidIDXGIFactory4)),
		uintptr(unsafe.Pointer(&factory))); hr != 0 || factory == nil {
		return nil
	}
	p.factory = factory
	return factory
}

// adapterVideoMemory reports whether adapter is the render adapter, and if
// so reads this process's VRAM usage from its IDXGIAdapter3 face.
func (p *Prober) adapterVideoMemory(adapter unsafe.Pointer) (matched bool, usage, budget uint64) {
	var desc dxgiAdapterDesc1
	if hr := comCall(adapter, slotGetDesc1, uintptr(unsafe.Pointer(&desc))); hr != 0 {
		return false, 0, 0
	}
	if p.vendor != 0 || p.device != 0 {
		if desc.vendorID != p.vendor || desc.deviceID != p.device {
			return false, 0, 0
		}
	} else if desc.dedicatedVideoMemory == 0 {
		return false, 0, 0 // software adapter (e.g. WARP) is not a GPU
	}

	var adapter3 unsafe.Pointer
	if hr := comCall(adapter, slotQueryInterface,
		uintptr(unsafe.Pointer(&iidIDXGIAdapter3)),
		uintptr(unsafe.Pointer(&adapter3))); hr != 0 || adapter3 == nil {
		return false, 0, 0
	}
	defer comRelease(adapter3)

	var info dxgiQueryVideoMemoryInfo
	const nodeZero = 0
	const segmentLocal = 0 // DXGI_MEMORY_SEGMENT_GROUP_LOCAL
	if hr := comCall(adapter3, slotQueryVideoMemoryInfo,
		nodeZero, segmentLocal, uintptr(unsafe.Pointer(&info))); hr != 0 {
		return false, 0, 0
	}
	return true, info.currentUsage, info.budget
}

// --- GPU utilization (PDH) ---

const (
	pdhFmtDouble  = 0x00000200
	pdhMoreData   = 0x800007D2
	pdhItemSizeW  = 24 // LPWSTR name + PDH_FMT_COUNTERVALUE (4 + 4 pad + 8 union)
	pdhMaxPercent = 100
)

// pdhItemDouble reads one PDH_FMT_COUNTERVALUE_ITEM (double flavour) out of
// the returned buffer.
func pdhItemDouble(buf []byte, i int) (string, float64, uint32) {
	base := i * pdhItemSizeW
	name := *(**uint16)(unsafe.Pointer(&buf[base]))
	status := *(*uint32)(unsafe.Pointer(&buf[base+8]))
	value := *(*float64)(unsafe.Pointer(&buf[base+16]))
	return utf16ToString(name), value, status
}

func utf16ToString(s *uint16) string {
	if s == nil {
		return ""
	}
	var runes []uint16
	for i := uintptr(0); ; i++ {
		u := *(*uint16)(unsafe.Add(unsafe.Pointer(s), i*2))
		if u == 0 {
			break
		}
		runes = append(runes, u)
	}
	return syscall.UTF16ToString(runes)
}

// gpuUsage aggregates the GPU engine counters: every instance sums into the
// system reading, instances tagged with this pid into the process reading.
// The counter path is locale-independent (PdhAddEnglishCounterW), and the
// sums cap at 100 the way Task Manager presents a GPU at full saturation.
func (p *Prober) gpuUsage() (process, system float64, ok bool) {
	if !p.pdhOK {
		return 0, 0, false
	}
	if status, _, _ := procPdhCollect.Call(p.pdhQuery); status != 0 {
		return 0, 0, false
	}

	var size, count uint32
	status, _, _ := procPdhFormatArray.Call(p.pdhCounter, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)), 0)
	if status != pdhMoreData && status != 0 {
		return 0, 0, false
	}
	if size == 0 || count == 0 {
		return 0, 0, false
	}
	buf := make([]byte, size)
	status, _, _ = procPdhFormatArray.Call(p.pdhCounter, pdhFmtDouble,
		uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&buf[0])))
	if status != 0 {
		return 0, 0, false
	}

	pidTag := "pid_" + strconv.FormatUint(uint64(p.pid), 10) + "_"
	for i := 0; i < int(count); i++ {
		name, value, cstatus := pdhItemDouble(buf, i)
		if cstatus != 0 || value < 0 {
			continue
		}
		system += value
		if strings.Contains(name, pidTag) {
			process += value
		}
	}
	return min(process, pdhMaxPercent), min(system, pdhMaxPercent), true
}
