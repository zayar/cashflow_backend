package models

import (
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func (u *User) AfterCreate(tx *gorm.DB) (err error) {
	if u.Role == UserRoleCustom {
		return createHistory(tx, "REGISTER", u.ID, "users", nil, u, "created custom user")
	}

	var history History
	history.BusinessId = u.BusinessId
	history.ActionType = "REGISTER"
	history.ReferenceID = u.ID
	history.ReferenceType = "users"
	history.Description = "created admin user"

	// create history
	if err := tx.Create(&history).Error; err != nil {
		return err
	}

	// clearing cache
	if err := utils.ClearRedisAdmin[User](); err != nil {
		return err
	}

	return nil
}

func (u *User) BeforeUpdate(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryUpdate(tx, u.ID, u, "Updated User"); err != nil {
		return err
	}
	// clearing cache
	if err := utils.ClearRedisAdmin[User](); err != nil {
		return err
	}

	return nil
}

func (u *User) AfterDelete(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryDelete(tx, u.ID, u, "Deleted User"); err != nil {
		return err
	}
	// clearing cache
	if err := utils.ClearRedisAdmin[User](); err != nil {
		return err
	}

	return nil
}

func (s *State) AfterCreate(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryCreate(tx, s.ID, s, "Created State"); err != nil {
		return err
	}
	// clearing cache
	if err := s.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (s *State) BeforeUpdate(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryUpdate(tx, s.ID, s, "Updated State"); err != nil {
		return err
	}
	// clearing cache
	if err := s.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (s *State) AfterDelete(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryDelete(tx, s.ID, s, "Deleted State"); err != nil {
		return err
	}
	// clearing cache
	if err := s.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (t *Township) AfterCreate(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryCreate(tx, t.ID, t, "Created Township"); err != nil {
		return err
	}
	// clearing cache
	if err := t.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (t *Township) BeforeUpdate(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryUpdate(tx, t.ID, t, "Updated Township"); err != nil {
		return err
	}
	// clearing cache
	if err := t.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}

func (t *Township) AfterDelete(tx *gorm.DB) (err error) {
	// creating history
	if err := SaveHistoryDelete(tx, t.ID, t, "Deleted Township"); err != nil {
		return err
	}
	// clearing cache
	if err := t.RemoveAllRedis(); err != nil {
		return err
	}

	return nil
}
