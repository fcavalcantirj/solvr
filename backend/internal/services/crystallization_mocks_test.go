package services

import (
	"context"
	"io"
)

// IPFS and CID test doubles shared by the crystallization tests, kept from the legacy
// CrystallizationService tests retired with it (idx 68).

// mockCrystallizationCIDSetter implements CrystallizationCIDSetter for testing.
type mockCrystallizationCIDSetter struct {
	calledWith struct {
		postID string
		cid    string
	}
	err error
}

func (m *mockCrystallizationCIDSetter) SetCrystallizationCID(ctx context.Context, postID, cid string) error {
	m.calledWith.postID = postID
	m.calledWith.cid = cid
	return m.err
}

// mockIPFSAdder implements IPFSContentAdder for testing.
type mockIPFSAdder struct {
	cid     string
	err     error
	content []byte // captures what was written
}

func (m *mockIPFSAdder) Add(ctx context.Context, reader io.Reader) (string, error) {
	if reader != nil {
		data, _ := io.ReadAll(reader)
		m.content = data
	}
	if m.err != nil {
		return "", m.err
	}
	return m.cid, nil
}

// mockIPFSPinner implements IPFSContentPinner for testing.
type mockIPFSPinner struct {
	pinnedCIDs []string
	err        error
}

func (m *mockIPFSPinner) Pin(ctx context.Context, cid string) error {
	if m.err != nil {
		return m.err
	}
	m.pinnedCIDs = append(m.pinnedCIDs, cid)
	return nil
}
