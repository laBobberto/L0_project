package validator

import "sync"

package validator

import (
"github.com/go-playground/validator/v10"
"sync"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// getInstance возвращает синглтон-экземпляр валидатора.
func getInstance() *validator.Validate {
	once.Do(func() {
		validate = validator.New()
	})
	return validate
}

// ValidateStruct выполняет валидацию по тегам структуры.
func ValidateStruct(s interface{}) error {
	return getInstance().Struct(s)
}