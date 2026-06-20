package backup

import (
	"reflect"
	"testing"
)

func sampleEnvelope() Envelope {
	return Envelope{
		Datasets: []string{
			`{"schemaVersion":1,"accounts":[{"id":"a1"}]}`,
			`{"schemaVersion":1,"accounts":[{"id":"b1"},{"id":"b2"}]}`,
		},
		WorkspaceRegistry: `{"active":"w1","workspaces":[{"id":"w1","name":"Personal"},{"id":"w2","name":"Side business"}]}`,
		Appearance:        Appearance{Theme: `{"accent":"#2e8b57"}`, Fonts: "data:font/woff2;base64,AA==", Banner: "data:image/png;base64,BB==", Prefs: `{"compact":true}`},
	}
}

func TestEnvelopeRoundTrip(t *testing.T) {
	in := sampleEnvelope()
	blob, err := MarshalEnvelope(in)
	if err != nil {
		t.Fatalf("MarshalEnvelope: %v", err)
	}
	out, err := UnmarshalEnvelope(blob)
	if err != nil {
		t.Fatalf("UnmarshalEnvelope: %v", err)
	}
	// Marshal stamps the current schema version; compare with that set.
	in.SchemaVersion = EnvelopeSchemaVersion
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip changed the envelope:\n in = %+v\nout = %+v", in, out)
	}
	if len(out.Datasets) != 2 || out.WorkspaceRegistry == "" || out.Appearance.Fonts == "" {
		t.Errorf("round-trip dropped data: %+v", out)
	}
}

func TestUnmarshalEnvelopeVersioning(t *testing.T) {
	// Unversioned (v0) is accepted as v1.
	v0, err := UnmarshalEnvelope([]byte(`{"datasets":["{}"]}`))
	if err != nil || v0.SchemaVersion != 1 {
		t.Errorf("unversioned envelope: ver=%d err=%v", v0.SchemaVersion, err)
	}
	// A newer-than-supported version is rejected, not silently mishandled.
	if _, err := UnmarshalEnvelope([]byte(`{"schemaVersion":999,"datasets":[]}`)); err == nil {
		t.Error("a newer schema version should be rejected")
	}
}

func TestIsEnvelope(t *testing.T) {
	env, _ := MarshalEnvelope(sampleEnvelope())
	if !IsEnvelope(env) {
		t.Error("a full-backup envelope should be detected as one")
	}
	// A single-workspace dataset (no datasets array) is NOT an envelope.
	dataset := []byte(`{"schemaVersion":1,"accounts":[],"transactions":[]}`)
	if IsEnvelope(dataset) {
		t.Error("a single dataset must not be mistaken for a full-backup envelope")
	}
	if IsEnvelope([]byte("not json")) {
		t.Error("garbage is not an envelope")
	}
}
