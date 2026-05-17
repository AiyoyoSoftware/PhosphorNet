package protocol

import "testing"

func TestDefaultHelloAdvertisesCurrentCompatibility(t *testing.T) {
	hello := DefaultHello("ed25519:client")
	if err := ValidateClientHello(hello); err != nil {
		t.Fatalf("ValidateClientHello() error = %v", err)
	}
	if hello.RuntimeProtocolVersion != RuntimeContractVersion {
		t.Fatalf("runtime protocol = %q, want %q", hello.RuntimeProtocolVersion, RuntimeContractVersion)
	}
	if hello.JSONUISchemaVersion != JSONUIContractVersion {
		t.Fatalf("JSON UI schema = %q, want %q", hello.JSONUISchemaVersion, JSONUIContractVersion)
	}
}

func TestValidateClientHelloRejectsOlderClient(t *testing.T) {
	hello := DefaultHello("ed25519:client")
	hello.RuntimeProtocolVersion = "phosphornet.door.runtime.v0"

	err := ValidateClientHello(hello)
	if err == nil {
		t.Fatal("ValidateClientHello() error = nil, want incompatibility")
	}
	if got := ErrorCodeOf(err); got != ErrorClientIncompatible {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, ErrorClientIncompatible)
	}
}

func TestValidateClientHelloRejectsMissingComponent(t *testing.T) {
	hello := DefaultHello("ed25519:client")
	hello.SupportedComponents = hello.SupportedComponents[:len(hello.SupportedComponents)-1]

	err := ValidateClientHello(hello)
	if err == nil {
		t.Fatal("ValidateClientHello() error = nil, want incompatibility")
	}
	if got := ErrorCodeOf(err); got != ErrorClientIncompatible {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q", got, ErrorClientIncompatible)
	}
}
