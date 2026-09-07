package app

import (
	"errors"
	"testing"
)

func TestValidateEstateDigitalMetadataRejectsCredentialPatterns(t *testing.T) {
	cases := []EstateDigitalAsset{
		{ServiceName: "Bank", Notes: "password: hunter2"},
		{ServiceName: "Bank", Notes: "pwd: hunter2"},
		{ServiceName: "Bank", Notes: "PIN: 1234"},
		{ServiceName: "Bank", Notes: "seed phrase: alpha beta gamma"},
		{ServiceName: "Bank", Notes: "private key in the notes"},
		{ServiceName: "Bank", Notes: "recovery code: 1a2b3c"},
		{ServiceName: "Bank", AccountIdentifier: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7"},
		{ServiceName: "Bank", Notes: "0123456789abcdef0123456789abcdef"},
		{ServiceName: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe"},
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7"},
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe", Notes: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7"},
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe", Notes: "0123456789abcdef0123456789abcdef"},
	}
	for i, item := range cases {
		if err := validateEstateDigitalMetadata(item); !errors.Is(err, ErrCredentialDetected) {
			t.Fatalf("case %d: expected ErrCredentialDetected, got %v for %#v", i, err, item)
		}
	}
}

func TestValidateEstateDigitalMetadataAllowsNonSecrets(t *testing.T) {
	cases := []EstateDigitalAsset{
		{ServiceName: "Password manager", AccountIdentifier: "family vault", AccessInstructionsLocation: "sealed envelope location", Notes: "no credentials stored"},
		{ServiceName: "Password manager", AccountIdentifier: "Family vault metadata", AccessInstructionsLocation: "Sealed envelope in safe", Notes: "No credentials stored"},
		{ServiceName: "Bank", AccountIdentifier: "checking-1234", AccessInstructionsLocation: "home safe", Notes: "statement copies"},
		{ServiceName: "Email", AccountIdentifier: "user@example.com", AccessInstructionsLocation: "safe deposit box", Notes: ""},
	}
	for i, item := range cases {
		if err := validateEstateDigitalMetadata(item); err != nil {
			t.Fatalf("case %d: unexpected rejection %v for %#v", i, err, item)
		}
	}
}
