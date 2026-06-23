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

// AuthInstance is one configured instance in `cfl auth list`: its key and an
// optional alias. The token is never represented.
type AuthInstance struct {
	Key   string  `json:"key"`
	Alias *string `json:"alias"`
}

// AuthList is the output of `cfl auth list`: the configured instances (key +
// optional alias) only, never any token.
type AuthList struct {
	Instances []AuthInstance `json:"instances"`
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

// NewAuthList builds an AuthList from configured instance keys and a key→alias
// map, ensuring a non-nil slice so it serializes as [] when empty. An instance
// without an alias gets a null alias.
func NewAuthList(keys []string, aliases map[string]string) AuthList {
	instances := make([]AuthInstance, 0, len(keys))
	for _, k := range keys {
		inst := AuthInstance{Key: k}
		if a, ok := aliases[k]; ok && a != "" {
			alias := a
			inst.Alias = &alias
		}
		instances = append(instances, inst)
	}
	return AuthList{Instances: instances}
}
