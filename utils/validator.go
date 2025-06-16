package utils

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func ValidateStruct(s interface{}) error {
	// Register custom validators if needed
	// validate.RegisterValidation("custom", func(fl validator.FieldLevel) bool { ... })

	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	// Format validation errors
	var errors []string
	for _, err := range err.(validator.ValidationErrors) {
		field := strings.ToLower(err.Field())
		tag := err.Tag()
		param := err.Param()

		switch tag {
		case "required":
			errors = append(errors, field+" is required")
		case "min":
			errors = append(errors, field+" must be at least "+param+" characters")
		case "max":
			errors = append(errors, field+" must be at most "+param+" characters")
		case "email":
			errors = append(errors, field+" must be a valid email")
		case "len":
			errors = append(errors, field+" must be exactly "+param+" characters")
		default:
			errors = append(errors, field+" is invalid")
		}
	}

	return fmt.Errorf(strings.Join(errors, ", "))
}