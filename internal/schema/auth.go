package schema

import (
	"encoding/json"
	"fmt"
)

// WhoAmI is the output of `cfl auth whoami`. It carries the resolved instance
// host plus the identity the instance returned. displayName is null when the
// instance does not provide one. The token is never represented here.
type WhoAmI struct {
	Host        string  `json:"host"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
}

// AuthList is the output of `cfl auth list`: the configured instance keys only,
// never any token.
type AuthList struct {
	Instances []string `json:"instances"`
}

// MapWhoAmI maps a Confluence current-user response into a WhoAmI for the given
// resolved host.
func MapWhoAmI(host string, raw []byte) (WhoAmI, error) {
	var wire struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return WhoAmI{}, fmt.Errorf("decode current user: %w", err)
	}
	w := WhoAmI{Host: host, Username: wire.Username}
	if wire.DisplayName != "" {
		dn := wire.DisplayName
		w.DisplayName = &dn
	}
	return w, nil
}

// NewAuthList builds an AuthList from configured instance keys, ensuring a
// non-nil slice so it serializes as [] when empty.
func NewAuthList(keys []string) AuthList {
	if keys == nil {
		keys = []string{}
	}
	return AuthList{Instances: keys}
}
