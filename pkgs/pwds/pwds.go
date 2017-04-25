/*
Package pwds is used to create and validate bcrypt passwords.
This is just a wrapper aroung golang.org/x/crypto/bcrypt that makes it easier to use.
*/

package pwds

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

//bcryptCost is how compute intensive it is to calculate a hash from a password
//the higher the value, the more compute resources are needed
//higher value means a password is harder to brute force but also longer to validate for user log ins
const bcryptCost = 12

//ErrInvalidPassword is thrown when a password and a given hash do not validate
var ErrInvalidPassword = errors.New("invalidPassword")

//Create creates a password hash from a supplied plain text password
func Create(input string) string {
	raw := []byte(input)
	hash, _ := bcrypt.GenerateFromPassword(raw, bcryptCost)

	return string(hash)
}

//Verify checks if a given password is correct against a provided hash
func Verify(rawPassword string, hashedPassword string) (bool, error) {
	rawPasswordByte := []byte(rawPassword)
	hashPasswordByte := []byte(hashedPassword)

	err := bcrypt.CompareHashAndPassword(hashPasswordByte, rawPasswordByte)
	if err != nil {
		return false, ErrInvalidPassword
	}

	//password verified
	return true, nil
}
