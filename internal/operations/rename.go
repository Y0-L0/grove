package operations

import (
	"errors"
	"fmt"
)

// Rename checks fork policy before moving a workspace and its records.
func (s *Service) Rename(oldName, newName string) error {
	ws, err := s.Store.GetWorkspace(oldName)
	if err != nil {
		return err
	}
	if ws == nil {
		return fmt.Errorf("workspace %s not found", oldName)
	}
	for _, h := range s.hooks() {
		if policy, ok := h.(RenamePolicy); ok {
			if err := policy.CheckRename(*ws, newName); err != nil {
				return err
			}
		}
	}
	renamer, ok := s.Workspace.(interface{ Rename(string, string) error })
	if !ok {
		return errors.New("workspace implementation does not support rename")
	}
	if err := renamer.Rename(oldName, newName); err != nil {
		return err
	}
	current, err := s.Store.GetWorkspace(newName)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("renamed workspace %s disappeared", newName)
	}
	for _, h := range s.hooks() {
		if policy, ok := h.(RenamePolicy); ok {
			if err := policy.RenameRecords(*current, oldName); err != nil {
				return errors.Join(err, renamer.Rename(newName, oldName))
			}
		}
	}
	return nil
}
