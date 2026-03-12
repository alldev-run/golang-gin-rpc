package jwtx

import "time"

func Logout(token string) error {

	claims, err := decodeClaims(token)
	if err != nil {
		return err
	}

	ttl := time.Until(claims.ExpireAt)

	if config.Store != nil {

		return config.Store.Set(
			"blacklist:"+claims.TokenID,
			"1",
			ttl,
		)
	}

	return nil
}

func RevokeUser(userID string) {

	if config.Store == nil {
		return
	}

	key := "user:version:" + userID

	config.Store.Set(key, "999", 0)
}
