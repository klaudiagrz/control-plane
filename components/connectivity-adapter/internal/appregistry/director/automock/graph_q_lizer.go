// Code generated by mockery v1.0.0. DO NOT EDIT.

package automock

import graphql "github.com/kyma-incubator/compass/components/director/pkg/graphql"
import mock "github.com/stretchr/testify/mock"

// GraphQLizer is an autogenerated mock type for the GraphQLizer type
type GraphQLizer struct {
	mock.Mock
}

// APIDefinitionInputToGQL provides a mock function with given fields: in
func (_m *GraphQLizer) APIDefinitionInputToGQL(in graphql.APIDefinitionInput) (string, error) {
	ret := _m.Called(in)

	var r0 string
	if rf, ok := ret.Get(0).(func(graphql.APIDefinitionInput) string); ok {
		r0 = rf(in)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(graphql.APIDefinitionInput) error); ok {
		r1 = rf(in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DocumentInputToGQL provides a mock function with given fields: in
func (_m *GraphQLizer) DocumentInputToGQL(in *graphql.DocumentInput) (string, error) {
	ret := _m.Called(in)

	var r0 string
	if rf, ok := ret.Get(0).(func(*graphql.DocumentInput) string); ok {
		r0 = rf(in)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*graphql.DocumentInput) error); ok {
		r1 = rf(in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// EventDefinitionInputToGQL provides a mock function with given fields: in
func (_m *GraphQLizer) EventDefinitionInputToGQL(in graphql.EventDefinitionInput) (string, error) {
	ret := _m.Called(in)

	var r0 string
	if rf, ok := ret.Get(0).(func(graphql.EventDefinitionInput) string); ok {
		r0 = rf(in)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(graphql.EventDefinitionInput) error); ok {
		r1 = rf(in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PackageCreateInputToGQL provides a mock function with given fields: in
func (_m *GraphQLizer) PackageCreateInputToGQL(in graphql.PackageCreateInput) (string, error) {
	ret := _m.Called(in)

	var r0 string
	if rf, ok := ret.Get(0).(func(graphql.PackageCreateInput) string); ok {
		r0 = rf(in)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(graphql.PackageCreateInput) error); ok {
		r1 = rf(in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PackageUpdateInputToGQL provides a mock function with given fields: in
func (_m *GraphQLizer) PackageUpdateInputToGQL(in graphql.PackageUpdateInput) (string, error) {
	ret := _m.Called(in)

	var r0 string
	if rf, ok := ret.Get(0).(func(graphql.PackageUpdateInput) string); ok {
		r0 = rf(in)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(graphql.PackageUpdateInput) error); ok {
		r1 = rf(in)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}