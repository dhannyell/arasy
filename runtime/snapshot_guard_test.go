package runtime

import (
	"reflect"
	"testing"
)

// TestDriverSnapshotFieldCoverage mirrors
// scene.TestChecksumCoversAllComponentStores: a field added to
// driverSnapshot without wiring it into captureSnapshot/restoreSnapshot
// corrupts rewind/replay in silence (a field present on the live driver but
// missing from the snapshot, or restored to its zero value). Bumping
// driverSnapshotFields is the forcing function that makes the omission loud.
func TestDriverSnapshotFieldCoverage(t *testing.T) {
	got := reflect.TypeFor[driverSnapshot]().NumField()
	if got != driverSnapshotFields {
		t.Fatalf("driverSnapshot has %d fields, driverSnapshotFields = %d — "+
			"wire the new/removed field into captureSnapshot AND restoreSnapshot, then update driverSnapshotFields",
			got, driverSnapshotFields)
	}
}
