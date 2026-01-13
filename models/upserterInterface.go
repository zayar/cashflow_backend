package models

import (
	"context"
	"fmt"
	"slices"

	"gorm.io/gorm"
)

type HasIsDeleted struct {
	IsDeletedItem bool `json:"is_deleted_item"`
}

func (i HasIsDeleted) IsDeleted() bool {
	return i.IsDeletedItem
}

// insert/do nothing, delete left ids
// upsert ids in input, delete left ids
func UpsertJoinTable(tx *gorm.DB, tableName string, primaryColumn string, secondaryColumn string, primaryId int, associatedIds []int) error {

	// construct values part => `(1,2), (1,4)`
	if len(associatedIds) <= 0 {
		deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = %d", tableName, primaryColumn, primaryId)
		if err := tx.Exec(deleteQuery).Error; err != nil {
			return err
		}
		return nil
	}

	valuesStr := fmt.Sprintf("(%d, %d)", primaryId, associatedIds[0])
	for i := 1; i < len(associatedIds); i++ {
		// not escaping values since all ids are legit integers
		valuesStr += fmt.Sprintf(", (%d, %d)", primaryId, associatedIds[i])
	}
	// upsert by insert ignore
	upsertQuery := fmt.Sprintf("INSERT IGNORE INTO %s (%s, %s) VALUES %s;", tableName, primaryColumn, secondaryColumn, valuesStr)
	if err := tx.Exec(upsertQuery).Error; err != nil {
		return err
	}
	// DELETE FROM $tableName WHERE $primaryColumn = $primaryId AND $secondaryColumn IN ?
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s = %d AND %s NOT IN ?", tableName, primaryColumn, primaryId, secondaryColumn)
	if err := tx.Exec(deleteQuery, associatedIds).Error; err != nil {
		return err
	}
	// save history
	return nil
}

type Replacer interface {
	Identifier // getID() int
	fillable() map[string]interface{}
}

// upsert one-to-many association, insert new, update old, delete left ids
func ReplaceAssociation[T Replacer](ctx context.Context,
	tx *gorm.DB, input []T, cond string, vars ...interface{}) error {

	var v T
	var validIds []int
	if err := tx.WithContext(ctx).
		Model(&v).
		Where(cond, vars...).
		Pluck("id", &validIds).Error; err != nil {
		return err
	}

	var updates []T
	var inserts []T

	for _, assoc := range input {

		// update
		if assoc.GetId() > 0 {
			// if id exists and is valid
			if index := slices.Index(validIds, assoc.GetId()); index >= 0 {
				// update
				updates = append(updates, assoc)
				// remove id from slice which will be cleared after
				validIds = append(validIds[:index], validIds[index+1:]...)
				continue
			}
		}
		inserts = append(inserts, assoc)
	}

	// do inserts
	if len(inserts) > 0 {
		if err := tx.WithContext(ctx).Omit("id").Create(&inserts).Error; err != nil {
			return err
		}
	}
	// updates
	if len(updates) > 0 {
		for _, update := range updates {
			var currentValue T
			// fetch before updating
			if err := tx.First(&currentValue, update.GetId()).Error; err != nil {
				return err
			}
			if err := tx.WithContext(ctx).Model(&currentValue).Updates(update.fillable()).Error; err != nil {
				return err
			}
		}
	}
	// delete ids left/not included in input
	if len(validIds) > 0 {
		if err := tx.WithContext(ctx).Where("id IN ?", validIds).Delete(&v).Error; err != nil {
			return err
		}
	}
	// TODO: create upsert history
	return nil
}

// delete other associated ids not include in input
//  tx.Exec(query).Error
// // 1, 2
// sqlTemplate := "INSERT IGNORE INTO %s (%s, %s) VALUES %s;"
// start := fmt.Sprintf("(%d, ?)", primaryId)
// end := strings.Repeat(", " + start, len(associatedIds)-1)
// valuesStr := start+end

// sql := fmt.Sprintf(sqlTemplate, tableName, primaryColumn, secondaryColumn, valuesStr)

// if err := tx.Exec(sql, associatedIds...); err != nil {
// 	return err
// }

/* INSERT IGNORE INTO
	sakila.new_customer (customer_id, store_id, first_name, last_name, email)
VALUES
	(8, 2, 'Susan', 'Wilson', 'susan.wilson@sakilacustomer.org'),
	(16, 1, 'Jane', 'Harrison', 'jane.harrison@sakilacustomer.org'),
	(17, 2, 'Bob', 'Johnson', 'bob.johnson@sakilacustomer.org');
*/

// Image, Document, ContactPerson
type Upserter interface {
	Store(tx *gorm.DB, ctx context.Context) error
	Delete(tx *gorm.DB, ctx context.Context) error
	Update(tx *gorm.DB, ctx context.Context, fillable map[string]interface{}) error
}

// NewImage, NewDocument, NewContactPerson
type Upsertable[ReturnType any] interface {
	Fillable() (map[string]interface{}, error)                          // for updates
	MapInput(referenceType string, referenceId int) (ReturnType, error) // for create
	IsDeleted() bool
	Identifier
}

// upsert input array, insert new, update existing, delete if flagged as isDeletedItem
func UpsertPolymorphicAssociation[ReturnType Upserter, InputType Upsertable[ReturnType]](
	ctx context.Context, tx *gorm.DB, inputSlice []InputType, referenceType string, referenceId int) ([]ReturnType, error) {

	var existingIds []int
	var temp ReturnType
	if err := tx.WithContext(ctx).
		Model(&temp).Where("reference_type = ? AND reference_id = ?", referenceType, referenceId).
		Select("id").Scan(&existingIds).Error; err != nil {
		return nil, err
	}

	var associations []ReturnType
	for _, input := range inputSlice {
		var item ReturnType
		id := input.GetId()

		// if item exists
		if slices.Contains(existingIds, id) {

			// fetch before update/delete
			if err := tx.WithContext(ctx).First(&item, id).Error; err != nil {
				return nil, err
			}

			// delete if input's isDeletedItem field is true
			if input.IsDeleted() {
				if err := item.Delete(tx, ctx); err != nil {
					return nil, err
				}
				// continue next iteration, skipping the appending
				continue

			} else {
				// update otherwise
				update, err := input.Fillable()
				if err != nil {
					return nil, err
				}

				if err := item.Update(tx, ctx, update); err != nil {
					return nil, err
				}
			}
		} else { // insert if id does not exist

			// don't insert if input is to be deleted
			if input.IsDeleted() {
				continue
			}
			// insert new item
			item, err := input.MapInput(referenceType, referenceId)
			if err != nil {
				return nil, err
			}
			if err := item.Store(tx, ctx); err != nil {
				return nil, err
			}
		}
		// append to slice after upserting item
		associations = append(associations, item)
	}

	return associations, nil
}
