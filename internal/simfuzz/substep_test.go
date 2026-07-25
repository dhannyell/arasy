package simfuzz

import (
	"testing"

	"github.com/dhannyell/arasy/scene"
)

func TestDecodeSubstepScenarioPreservesFieldOrder(t *testing.T) {
	data := []byte{8, 20, 11, 0x05, 0x00, 5, 0, 0, 0}
	scenario := decodeSubstepScenario(data)
	world := scene.NewWorld()
	var ids []scene.EntityID
	SpawnFromScript(world, scenario.Script, fuzzBounds, &ids)
	if len(ids) != 2 {
		t.Fatalf("spawned %d entities, want projectile and wall", len(ids))
	}

	body, ok := world.Body(ids[0])
	if !ok {
		t.Fatal("projectile missing Body")
	}
	if body.InvMass != 1 || body.Restitution != 0 {
		t.Fatalf("projectile body = %+v, want InvMass=1 Restitution=0", body)
	}

	collider, ok := world.CircleCollider(ids[0])
	if !ok {
		t.Fatal("projectile missing CircleCollider")
	}
	const wantRadius = 0.05 + 11.0/255.0*0.5
	if collider.Radius != wantRadius {
		t.Fatalf("projectile radius = %f, want %f", collider.Radius, wantRadius)
	}

	ballistics, ok := world.Ballistics(ids[0])
	if !ok {
		t.Fatal("projectile missing Ballistics")
	}
	if ballistics.Substeps != 8 {
		t.Fatalf("projectile substeps = %d, want 8", ballistics.Substeps)
	}
}
