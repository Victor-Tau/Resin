//go:build !linux

package localipv6

import "fmt"

func (m Mapping) Acquire() (func() error, error) {
	return nil, fmt.Errorf("automatic IPv6 address assignment requires Linux")
}
