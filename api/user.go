package api

import (
	"fmt"
	"time"

	"github.com/satori/go.uuid"
	"github.com/tecsisa/authorizr/database"
)

// TYPE DEFINITIONS

// User domain
type User struct {
	ID         string    `json:"id, omitempty"`
	ExternalID string    `json:"externalId, omitempty"`
	Path       string    `json:"path, omitempty"`
	CreateAt   time.Time `json:"createAt, omitempty"`
	Urn        string    `json:"urn, omitempty"`
}

func (u User) GetUrn() string {
	return u.Urn
}

// USER API IMPLEMENTATION

func (api AuthAPI) AddUser(authenticatedUser AuthenticatedUser, externalId string, path string) (*User, error) {
	// Validate fields
	if !IsValidUserExternalID(externalId) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: externalId %v", externalId),
		}
	}
	if !IsValidPath(path) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: path %v", path),
		}
	}

	user := createUser(externalId, path)

	// Check restrictions
	usersFiltered, err := api.GetAuthorizedUsers(authenticatedUser, user.Urn, USER_ACTION_CREATE_USER, []User{user})
	if err != nil {
		return nil, err
	}
	if len(usersFiltered) < 1 {
		return nil, &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, user.Urn),
		}
	}

	// Check if user already exists
	_, err = api.UserRepo.GetUserByExternalID(externalId)

	if err != nil {
		// Transform to DB error
		dbError := err.(*database.Error)
		// User doesn't exist in DB
		switch dbError.Code {
		case database.USER_NOT_FOUND:
			// Create user
			createdUser, err := api.UserRepo.AddUser(user)

			// Check unexpected DB error
			if err != nil {
				//Transform to DB error
				dbError := err.(*database.Error)
				return nil, &Error{
					Code:    UNKNOWN_API_ERROR,
					Message: dbError.Message,
				}
			}

			// Return user created
			return createdUser, nil
		default: // Unexpected error
			return nil, &Error{
				Code:    UNKNOWN_API_ERROR,
				Message: dbError.Message,
			}
		}
	} else {
		return nil, &Error{
			Code:    USER_ALREADY_EXIST,
			Message: fmt.Sprintf("Unable to create user, user with externalId %v already exist", externalId),
		}
	}

}

func (api AuthAPI) GetUserByExternalID(authenticatedUser AuthenticatedUser, externalId string) (*User, error) {
	if !IsValidUserExternalID(externalId) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: externalId %v", externalId),
		}
	}
	// Retrieve user from DB
	user, err := api.UserRepo.GetUserByExternalID(externalId)

	// Error handling
	if err != nil {
		//Transform to DB error
		dbError := err.(*database.Error)
		// User doesn't exist in DB
		if dbError.Code == database.USER_NOT_FOUND {
			return nil, &Error{
				Code:    USER_BY_EXTERNAL_ID_NOT_FOUND,
				Message: dbError.Message,
			}
		} else { // Unexpected error
			return nil, &Error{
				Code:    UNKNOWN_API_ERROR,
				Message: dbError.Message,
			}
		}
	}

	// Check restrictions
	filteredUsers, err := api.GetAuthorizedUsers(authenticatedUser, user.Urn, USER_ACTION_GET_USER, []User{*user})
	if err != nil {
		return nil, err
	}
	if len(filteredUsers) > 0 {
		filteredUser := filteredUsers[0]
		return &filteredUser, nil
	} else {
		return nil, &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, user.Urn),
		}
	}

}

func (api AuthAPI) ListUsers(authenticatedUser AuthenticatedUser, pathPrefix string) ([]string, error) {
	// Check parameters
	if len(pathPrefix) > 0 && !IsValidPath(pathPrefix) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: PathPrefix %v", pathPrefix),
		}
	}

	if len(pathPrefix) == 0 {
		pathPrefix = "/"
	}

	// Retrieve users with specified path prefix
	users, err := api.UserRepo.GetUsersFiltered(pathPrefix)

	// Error handling
	if err != nil {
		//Transform to DB error
		dbError := err.(*database.Error)
		return nil, &Error{
			Code:    UNKNOWN_API_ERROR,
			Message: dbError.Message,
		}
	}

	// Check restrictions
	urnPrefix := GetUrnPrefix("", RESOURCE_USER, pathPrefix)
	usersFiltered, err := api.GetAuthorizedUsers(authenticatedUser, urnPrefix, USER_ACTION_LIST_USERS, users)
	if err != nil {
		return nil, err
	}

	// Return user IDs
	externalIds := []string{}
	for _, u := range usersFiltered {
		externalIds = append(externalIds, u.ExternalID)
	}

	return externalIds, nil
}

func (api AuthAPI) UpdateUser(authenticatedUser AuthenticatedUser, externalId string, newPath string) (*User, error) {
	// Validate fields
	if !IsValidUserExternalID(externalId) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: externalId %v", externalId),
		}
	}
	if !IsValidPath(newPath) {
		return nil, &Error{
			Code:    INVALID_PARAMETER_ERROR,
			Message: fmt.Sprintf("Invalid parameter: path %v", newPath),
		}
	}

	// Call repo to retrieve the user
	userDB, err := api.GetUserByExternalID(authenticatedUser, externalId)
	if err != nil {
		return nil, err
	}

	// Check restrictions
	usersFiltered, err := api.GetAuthorizedUsers(authenticatedUser, userDB.Urn, USER_ACTION_UPDATE_USER, []User{*userDB})
	if err != nil {
		return nil, err
	}
	if len(usersFiltered) < 1 {
		return nil, &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, userDB.Urn),
		}
	}

	userToUpdate := createUser(externalId, newPath)

	// Check restrictions
	usersFiltered, err = api.GetAuthorizedUsers(authenticatedUser, userToUpdate.Urn, USER_ACTION_GET_USER, []User{userToUpdate})
	if err != nil {
		return nil, err
	}
	if len(usersFiltered) < 1 {
		return nil, &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, userToUpdate.Urn),
		}
	}

	// Update user
	user, err := api.UserRepo.UpdateUser(*userDB, newPath, userToUpdate.Urn)

	// Check unexpected DB error
	if err != nil {
		//Transform to DB error
		dbError := err.(*database.Error)
		return nil, &Error{
			Code:    UNKNOWN_API_ERROR,
			Message: dbError.Message,
		}
	}

	return user, nil

}

func (api AuthAPI) RemoveUser(authenticatedUser AuthenticatedUser, externalId string) error {
	// Call repo to retrieve the user
	user, err := api.GetUserByExternalID(authenticatedUser, externalId)
	if err != nil {
		return err
	}

	// Check restrictions
	usersFiltered, err := api.GetAuthorizedUsers(authenticatedUser, user.Urn, USER_ACTION_DELETE_USER, []User{*user})
	if err != nil {
		return err
	}
	if len(usersFiltered) < 1 {
		return &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, user.Urn),
		}
	}

	// Remove user with given id
	err = api.UserRepo.RemoveUser(user.ID)

	// Error handling
	if err != nil {
		//Transform to DB error
		dbError := err.(*database.Error)
		return &Error{
			Code:    UNKNOWN_API_ERROR,
			Message: dbError.Message,
		}
	}

	return nil
}

func (api AuthAPI) ListGroupsByUser(authenticatedUser AuthenticatedUser, externalId string) ([]GroupIdentity, error) {
	// Call repo to retrieve the user
	user, err := api.GetUserByExternalID(authenticatedUser, externalId)
	if err != nil {
		return nil, err
	}

	// Check restrictions
	usersFiltered, err := api.GetAuthorizedUsers(authenticatedUser, user.Urn, USER_ACTION_LIST_GROUPS_FOR_USER, []User{*user})
	if err != nil {
		return nil, err
	}
	if len(usersFiltered) < 1 {
		return nil, &Error{
			Code: UNAUTHORIZED_RESOURCES_ERROR,
			Message: fmt.Sprintf("User with externalId %v is not allowed to access to resource %v",
				authenticatedUser.Identifier, user.Urn),
		}
	}

	// Call group repo to retrieve groups associated to user
	groups, err := api.UserRepo.GetGroupsByUserID(user.ID)

	// Error handling
	if err != nil {
		//Transform to DB error
		dbError := err.(*database.Error)
		return nil, &Error{
			Code:    UNKNOWN_API_ERROR,
			Message: dbError.Message,
		}
	}

	// Transform to identifiers
	groupIDs := []GroupIdentity{}
	for _, g := range groups {
		groupIDs = append(groupIDs, GroupIdentity{
			Org:  g.Org,
			Name: g.Name,
		})
	}

	return groupIDs, nil
}

// PRIVATE HELPER METHODS

func createUser(externalId string, path string) User {
	urn := CreateUrn("", RESOURCE_USER, path, externalId)
	user := User{
		ID:         uuid.NewV4().String(),
		ExternalID: externalId,
		Path:       path,
		CreateAt:   time.Now().UTC(),
		Urn:        urn,
	}

	return user
}
