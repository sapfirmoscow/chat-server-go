package models

import "time"

type Chat struct {
	ID        string
	MemberIDs []string
	CreatedAt time.Time
}

func (c *Chat) HasMember(userID string) bool {
	for _, id := range c.MemberIDs {
		if id == userID {
			return true
		}
	}
	return false
}
