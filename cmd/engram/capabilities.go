package main

import (
	"fmt"
	"os"
)

type capabilityContract struct {
	SchemaVersion int                `json:"schema_version"`
	Features      capabilityFeatures `json:"features"`
}

type capabilityFeatures struct {
	AtomicTopicCAS atomicTopicCASCapability `json:"atomic_topic_cas"`
}

type atomicTopicCASCapability struct {
	Supported     bool                `json:"supported"`
	Input         atomicTopicCASInput `json:"input"`
	SuccessFields []string            `json:"success_fields"`
	ErrorCodes    []string            `json:"error_codes"`
}

type atomicTopicCASInput struct {
	ExpectedRevision expectedRevisionCapability `json:"expected_revision"`
}

type expectedRevisionCapability struct {
	Type     string `json:"type"`
	Minimum  int    `json:"minimum"`
	Requires string `json:"requires"`
}

func capabilitiesV1() capabilityContract {
	return capabilityContract{
		SchemaVersion: 1,
		Features: capabilityFeatures{
			AtomicTopicCAS: atomicTopicCASCapability{
				Supported: true,
				Input: atomicTopicCASInput{
					ExpectedRevision: expectedRevisionCapability{
						Type:     "integer",
						Minimum:  0,
						Requires: "topic_key",
					},
				},
				SuccessFields: []string{"id", "sync_id", "revision_count"},
				ErrorCodes: []string{
					"revision_conflict",
					"expected_revision_requires_topic",
					"invalid_expected_revision",
				},
			},
		},
	}
}

func cmdCapabilities(args []string) error {
	if len(args) == 0 {
		fmt.Println("Engram capability schema 1")
		fmt.Println("atomic_topic_cas: supported")
		return nil
	}
	if len(args) != 1 || args[0] != "--json" {
		return fmt.Errorf("usage: engram capabilities [--json]")
	}

	out, err := jsonMarshalIndent(capabilitiesV1(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode capabilities: %w", err)
	}
	_, err = fmt.Fprintln(os.Stdout, string(out))
	return err
}
