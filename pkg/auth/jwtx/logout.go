package jwtx

import "time"

// Logout invalidates a token by adding it to the blacklist.
func (m *Manager) Logout(token string) error {
	cfg := m.Config()
	claims, err := m.decodeClaims(token)
	if err != nil {
		return err
	}

	ttl := time.Until(claims.ExpireAt)

	if cfg.Store != nil {

		return cfg.Store.Set(
			"blacklist:"+claims.TokenID,
			"1",
			ttl,
		)
	}

	return nil
}

// RevokeUser revokes all tokens for a user by incrementing their version.
func (m *Manager) RevokeUser(userID string) {
	cfg := m.Config()
	if cfg.Store == nil {
		return
	}

	key := "user:version:" + userID

	cfg.Store.Set(key, "999", 0)
}

func Logout(token string) error {
	return DefaultManager().Logout(token)
}

func RevokeUser(userID string) {
	DefaultManager().RevokeUser(userID)
}
