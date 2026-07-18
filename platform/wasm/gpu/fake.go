package gpu

// Call records one Device operation in the order it was requested.
type Call struct {
	Method string
	Args   []any
}

// Fake is a recording Device adapter for native tests.
type Fake struct {
	Calls []Call
	next  int
}

type fakeHandle int

func (f *Fake) handle(method string, args ...any) Handle {
	f.Calls = append(f.Calls, Call{Method: method, Args: args})
	f.next++
	return fakeHandle(f.next)
}

func (f *Fake) record(method string, args ...any) {
	f.Calls = append(f.Calls, Call{Method: method, Args: args})
}

func (f *Fake) CreateShaderModule(code string) Handle { return f.handle("CreateShaderModule", code) }
func (f *Fake) CreateRenderPipeline(desc map[string]any) Handle {
	return f.handle("CreateRenderPipeline", desc)
}
func (f *Fake) CreateSampler(desc map[string]any) Handle   { return f.handle("CreateSampler", desc) }
func (f *Fake) CreateTexture(desc map[string]any) Handle   { return f.handle("CreateTexture", desc) }
func (f *Fake) CreateBuffer(desc map[string]any) Handle    { return f.handle("CreateBuffer", desc) }
func (f *Fake) CreateBindGroup(desc map[string]any) Handle { return f.handle("CreateBindGroup", desc) }
func (f *Fake) CreateView(texture Handle, desc map[string]any) Handle {
	return f.handle("CreateView", texture, desc)
}
func (f *Fake) BindGroupLayout(pipeline Handle, index int) Handle {
	return f.handle("BindGroupLayout", pipeline, index)
}
func (f *Fake) WriteBuffer(buffer Handle, offset int, data []byte) {
	f.record("WriteBuffer", buffer, offset, append([]byte(nil), data...))
}
func (f *Fake) WriteTexture(dst map[string]any, data []byte, layout, size map[string]any) {
	f.record("WriteTexture", dst, append([]byte(nil), data...), layout, size)
}
