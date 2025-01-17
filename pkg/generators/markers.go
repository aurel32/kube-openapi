/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package generators

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	defaultergen "k8s.io/gengo/examples/defaulter-gen/generators"
	"k8s.io/gengo/types"
	openapi "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

type CELTag struct {
	Rule              string `json:"rule,omitempty"`
	Message           string `json:"message,omitempty"`
	MessageExpression string `json:"messageExpression,omitempty"`
	OptionalOldSelf   *bool  `json:"optionalOldSelf,omitempty"`
	Reason            string `json:"reason,omitempty"`
	FieldPath         string `json:"fieldPath,omitempty"`
}

func (c *CELTag) Validate() error {
	if c == nil || *c == (CELTag{}) {
		return fmt.Errorf("empty CEL tag is not allowed")
	}

	var errs []error
	if c.Rule == "" {
		errs = append(errs, fmt.Errorf("rule cannot be empty"))
	}
	if c.Message == "" && c.MessageExpression == "" {
		errs = append(errs, fmt.Errorf("message or messageExpression must be set"))
	}
	if c.Message != "" && c.MessageExpression != "" {
		errs = append(errs, fmt.Errorf("message and messageExpression cannot be set at the same time"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// CommentTags represents the parsed comment tags for a given type. These types are then used to generate schema validations.
type CommentTags struct {
	spec.SchemaProps

	CEL []CELTag `json:"cel,omitempty"`

	// Future markers can all be parsed into this centralized struct...
	// Optional bool `json:"optional,omitempty"`
	// Default  any  `json:"default,omitempty"`
}

// validates the parameters in a CommentTags instance. Returns any errors encountered.
func (c CommentTags) Validate() error {

	var err error

	if c.MinLength != nil && *c.MinLength < 0 {
		err = errors.Join(err, fmt.Errorf("minLength cannot be negative"))
	}
	if c.MaxLength != nil && *c.MaxLength < 0 {
		err = errors.Join(err, fmt.Errorf("maxLength cannot be negative"))
	}
	if c.MinItems != nil && *c.MinItems < 0 {
		err = errors.Join(err, fmt.Errorf("minItems cannot be negative"))
	}
	if c.MaxItems != nil && *c.MaxItems < 0 {
		err = errors.Join(err, fmt.Errorf("maxItems cannot be negative"))
	}
	if c.MinProperties != nil && *c.MinProperties < 0 {
		err = errors.Join(err, fmt.Errorf("minProperties cannot be negative"))
	}
	if c.MaxProperties != nil && *c.MaxProperties < 0 {
		err = errors.Join(err, fmt.Errorf("maxProperties cannot be negative"))
	}
	if c.Minimum != nil && c.Maximum != nil && *c.Minimum > *c.Maximum {
		err = errors.Join(err, fmt.Errorf("minimum %f is greater than maximum %f", *c.Minimum, *c.Maximum))
	}
	if (c.ExclusiveMinimum || c.ExclusiveMaximum) && c.Minimum != nil && c.Maximum != nil && *c.Minimum == *c.Maximum {
		err = errors.Join(err, fmt.Errorf("exclusiveMinimum/Maximum cannot be set when minimum == maximum"))
	}
	if c.MinLength != nil && c.MaxLength != nil && *c.MinLength > *c.MaxLength {
		err = errors.Join(err, fmt.Errorf("minLength %d is greater than maxLength %d", *c.MinLength, *c.MaxLength))
	}
	if c.MinItems != nil && c.MaxItems != nil && *c.MinItems > *c.MaxItems {
		err = errors.Join(err, fmt.Errorf("minItems %d is greater than maxItems %d", *c.MinItems, *c.MaxItems))
	}
	if c.MinProperties != nil && c.MaxProperties != nil && *c.MinProperties > *c.MaxProperties {
		err = errors.Join(err, fmt.Errorf("minProperties %d is greater than maxProperties %d", *c.MinProperties, *c.MaxProperties))
	}
	if c.Pattern != "" {
		_, e := regexp.Compile(c.Pattern)
		if e != nil {
			err = errors.Join(err, fmt.Errorf("invalid pattern %q: %v", c.Pattern, e))
		}
	}
	if c.MultipleOf != nil && *c.MultipleOf == 0 {
		err = errors.Join(err, fmt.Errorf("multipleOf cannot be 0"))
	}

	for i, celTag := range c.CEL {
		celError := celTag.Validate()
		if celError == nil {
			continue
		}
		err = errors.Join(err, fmt.Errorf("invalid CEL tag at index %d: %w", i, celError))
	}

	return err
}

// Performs type-specific validation for CommentTags porameters. Accepts a Type instance and returns any errors encountered during validation.
func (c CommentTags) ValidateType(t *types.Type) error {
	var err error

	resolvedType := resolveAliasAndPtrType(t)
	typeString, _ := openapi.OpenAPITypeFormat(resolvedType.String()) // will be empty for complicated types
	isNoValidate := resolvedType.Kind == types.Interface || resolvedType.Kind == types.Struct

	if !isNoValidate {

		isArray := resolvedType.Kind == types.Slice || resolvedType.Kind == types.Array
		isMap := resolvedType.Kind == types.Map
		isString := typeString == "string"
		isInt := typeString == "integer"
		isFloat := typeString == "number"

		if c.MaxItems != nil && !isArray {
			err = errors.Join(err, fmt.Errorf("maxItems can only be used on array types"))
		}
		if c.MinItems != nil && !isArray {
			err = errors.Join(err, fmt.Errorf("minItems can only be used on array types"))
		}
		if c.UniqueItems && !isArray {
			err = errors.Join(err, fmt.Errorf("uniqueItems can only be used on array types"))
		}
		if c.MaxProperties != nil && !isMap {
			err = errors.Join(err, fmt.Errorf("maxProperties can only be used on map types"))
		}
		if c.MinProperties != nil && !isMap {
			err = errors.Join(err, fmt.Errorf("minProperties can only be used on map types"))
		}
		if c.MinLength != nil && !isString {
			err = errors.Join(err, fmt.Errorf("minLength can only be used on string types"))
		}
		if c.MaxLength != nil && !isString {
			err = errors.Join(err, fmt.Errorf("maxLength can only be used on string types"))
		}
		if c.Pattern != "" && !isString {
			err = errors.Join(err, fmt.Errorf("pattern can only be used on string types"))
		}
		if c.Minimum != nil && !isInt && !isFloat {
			err = errors.Join(err, fmt.Errorf("minimum can only be used on numeric types"))
		}
		if c.Maximum != nil && !isInt && !isFloat {
			err = errors.Join(err, fmt.Errorf("maximum can only be used on numeric types"))
		}
		if c.MultipleOf != nil && !isInt && !isFloat {
			err = errors.Join(err, fmt.Errorf("multipleOf can only be used on numeric types"))
		}
		if c.ExclusiveMinimum && !isInt && !isFloat {
			err = errors.Join(err, fmt.Errorf("exclusiveMinimum can only be used on numeric types"))
		}
		if c.ExclusiveMaximum && !isInt && !isFloat {
			err = errors.Join(err, fmt.Errorf("exclusiveMaximum can only be used on numeric types"))
		}
	}

	return err
}

// Parses the given comments into a CommentTags type. Validates the parsed comment tags, and returns the result.
// Accepts an optional type to validate against, and a prefix to filter out markers not related to validation.
// Accepts a prefix to filter out markers not related to validation.
// Returns any errors encountered while parsing or validating the comment tags.
func ParseCommentTags(t *types.Type, comments []string, prefix string) (CommentTags, error) {

	markers, err := parseMarkers(comments, prefix)
	if err != nil {
		return CommentTags{}, fmt.Errorf("failed to parse marker comments: %w", err)
	}
	nested, err := nestMarkers(markers)
	if err != nil {
		return CommentTags{}, fmt.Errorf("invalid marker comments: %w", err)
	}

	// Parse the map into a CommentTags type by marshalling and unmarshalling
	// as JSON in leiu of an unstructured converter.
	out, err := json.Marshal(nested)
	if err != nil {
		return CommentTags{}, fmt.Errorf("failed to marshal marker comments: %w", err)
	}

	var commentTags CommentTags
	if err = json.Unmarshal(out, &commentTags); err != nil {
		return CommentTags{}, fmt.Errorf("failed to unmarshal marker comments: %w", err)
	}

	// Validate the parsed comment tags
	validationErrors := commentTags.Validate()

	if t != nil {
		validationErrors = errors.Join(validationErrors, commentTags.ValidateType(t))
	}

	if validationErrors != nil {
		return CommentTags{}, fmt.Errorf("invalid marker comments: %w", validationErrors)
	}

	return commentTags, nil
}

var (
	allowedKeyCharacterSet = `[:_a-zA-Z0-9\[\]\-]`
	valueEmpty             = regexp.MustCompile(fmt.Sprintf(`^(%s*)$`, allowedKeyCharacterSet))
	valueAssign            = regexp.MustCompile(fmt.Sprintf(`^(%s*)=(.*)$`, allowedKeyCharacterSet))
	valueRawString         = regexp.MustCompile(fmt.Sprintf(`^(%s*)>(.*)$`, allowedKeyCharacterSet))
)

// extractCommentTags parses comments for lines of the form:
//
//	'marker' + "key=value"
//
//	or to specify truthy boolean keys:
//
//	'marker' + "key"
//
// Values are optional; "" is the default.  A tag can be specified more than
// one time and all values are returned.  Returns a map with an entry for
// for each key and a value.
//
// Similar to version from gengo, but this version support only allows one
// value per key (preferring explicit array indices), supports raw strings
// with concatenation, and limits the usable characters allowed in a key
// (for simpler parsing).
//
// Assignments and empty values have the same syntax as from gengo. Raw strings
// have the syntax:
//
//	'marker' + "key>value"
//	'marker' + "key>value"
//
// Successive usages of the same raw string key results in concatenating each
// line with `\n` in between. It is an error to use `=` to assing to a previously
// assigned key
// (in contrast to types.ExtractCommentTags which allows array-typed
// values to be specified using `=`).
func extractCommentTags(marker string, lines []string) (map[string]string, error) {
	out := map[string]string{}

	// Used to track the the line immediately prior to the one being iterated.
	// If there was an invalid or ignored line, these values get reset.
	lastKey := ""
	lastIndex := -1
	lastArrayKey := ""

	var lintErrors []error

	for _, line := range lines {
		line = strings.Trim(line, " ")

		// Track the current value of the last vars to use in this loop iteration
		// before they are reset for the next iteration.
		previousKey := lastKey
		previousArrayKey := lastArrayKey
		previousIndex := lastIndex

		// Make sure last vars gets reset if we `continue`
		lastIndex = -1
		lastArrayKey = ""
		lastKey = ""

		if len(line) == 0 {
			continue
		} else if !strings.HasPrefix(line, marker) {
			continue
		}

		line = strings.TrimPrefix(line, marker)

		key := ""
		value := ""

		if matches := valueAssign.FindStringSubmatch(line); matches != nil {
			key = matches[1]
			value = matches[2]

			// If key exists, throw error.
			// Some of the old kube open-api gen marker comments like
			// `+listMapKeys` allowed a list to be specified by writing key=value
			// multiple times.
			//
			// This is not longer supported for the prefixed marker comments.
			// This is to prevent confusion with the new array syntax which
			// supports lists of objects.
			//
			// The old marker comments like +listMapKeys will remain functional,
			// but new markers will not support it.
			if _, ok := out[key]; ok {
				return nil, fmt.Errorf("cannot have multiple values for key '%v'", key)
			}

		} else if matches := valueEmpty.FindStringSubmatch(line); matches != nil {
			key = matches[1]
			value = ""

		} else if matches := valueRawString.FindStringSubmatch(line); matches != nil {
			toAdd := strings.Trim(string(matches[2]), " ")

			key = matches[1]

			// First usage as a raw string.
			if existing, exists := out[key]; !exists {

				// Encode the raw string as JSON to ensure that it is properly escaped.
				valueBytes, err := json.Marshal(toAdd)
				if err != nil {
					return nil, fmt.Errorf("invalid value for key %v: %w", key, err)
				}

				value = string(valueBytes)
			} else if key != previousKey {
				// Successive usages of the same key of a raw string must be
				// consecutive
				return nil, fmt.Errorf("concatenations to key '%s' must be consecutive with its assignment", key)
			} else {
				// If it is a consecutive repeat usage, concatenate to the
				// existing value.
				//
				// Decode JSON string, append to it, re-encode JSON string.
				// Kinda janky but this is a code-generator...
				var unmarshalled string
				if err := json.Unmarshal([]byte(existing), &unmarshalled); err != nil {
					return nil, fmt.Errorf("invalid value for key %v: %w", key, err)
				} else {
					unmarshalled += "\n" + toAdd
					valueBytes, err := json.Marshal(unmarshalled)
					if err != nil {
						return nil, fmt.Errorf("invalid value for key %v: %w", key, err)
					}

					value = string(valueBytes)
				}
			}
		} else {
			// Comment has the correct prefix, but incorrect syntax, so it is an
			// error
			return nil, fmt.Errorf("invalid marker comment does not match expected `+key=<json formatted value>` pattern: %v", line)
		}

		out[key] = value
		lastKey = key

		// Lint the array subscript for common mistakes. This only lints the last
		// array index used, (since we do not have a need for nested arrays yet
		// in markers)
		if arrayPath, index, hasSubscript, err := extractArraySubscript(key); hasSubscript {
			// If index is non-zero, check that that previous line was for the same
			// key and either the same or previous index
			if err != nil {
				lintErrors = append(lintErrors, fmt.Errorf("error parsing %v: expected integer index in key '%v'", line, key))
			} else if previousArrayKey != arrayPath && index != 0 {
				lintErrors = append(lintErrors, fmt.Errorf("error parsing %v: non-consecutive index %v for key '%v'", line, index, arrayPath))
			} else if index != previousIndex+1 && index != previousIndex {
				lintErrors = append(lintErrors, fmt.Errorf("error parsing %v: non-consecutive index %v for key '%v'", line, index, arrayPath))
			}

			lastIndex = index
			lastArrayKey = arrayPath
		}
	}

	if len(lintErrors) > 0 {
		return nil, errors.Join(lintErrors...)
	}

	return out, nil
}

// Extracts and parses the given marker comments into a map of key -> value.
// Accepts a prefix to filter out markers not related to validation.
// The prefix is removed from the key in the returned map.
// Empty keys and invalid values will return errors, refs are currently unsupported and will be skipped.
func parseMarkers(markerComments []string, prefix string) (map[string]any, error) {
	markers, err := extractCommentTags(prefix, markerComments)
	if err != nil {
		return nil, err
	}

	// Parse the values as JSON
	result := map[string]any{}
	for key, value := range markers {
		var unmarshalled interface{}

		if len(key) == 0 {
			return nil, fmt.Errorf("cannot have empty key for marker comment")
		} else if _, ok := defaultergen.ParseSymbolReference(value, ""); ok {
			// Skip ref markers
			continue
		} else if len(value) == 0 {
			// Empty value means key is implicitly a bool
			result[key] = true
		} else if err := json.Unmarshal([]byte(value), &unmarshalled); err != nil {
			// Not valid JSON, throw error
			return nil, fmt.Errorf("failed to parse value for key %v as JSON: %w", key, err)
		} else {
			// Is is valid JSON, use as a JSON value
			result[key] = unmarshalled
		}
	}
	return result, nil
}

// Converts a map of:
//
//	"a:b:c": 1
//	"a:b:d": 2
//	"a:e": 3
//	"f": 4
//
// Into:
//
//	 map[string]any{
//	   "a": map[string]any{
//		      "b": map[string]any{
//		          "c": 1,
//				  "d": 2,
//			   },
//			   "e": 3,
//		  },
//		  "f": 4,
//	 }
//
// Returns a list of joined errors for any invalid keys. See putNestedValue for more details.
func nestMarkers(markers map[string]any) (map[string]any, error) {
	nested := make(map[string]any)
	var errs []error
	for key, value := range markers {
		var err error
		keys := strings.Split(key, ":")

		if err = putNestedValue(nested, keys, value); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return nested, nil
}

// Recursively puts a value into the given keypath, creating intermediate maps
// and slices as needed. If a key is of the form `foo[bar]`, then bar will be
// treated as an index into the array foo. If bar is not a valid integer, putNestedValue returns an error.
func putNestedValue(m map[string]any, k []string, v any) error {
	if len(k) == 0 {
		return nil
	}

	key := k[0]
	rest := k[1:]

	// Array case
	if arrayKeyWithoutSubscript, index, hasSubscript, err := extractArraySubscript(key); err != nil {
		return fmt.Errorf("error parsing subscript for key %v: %w", key, err)
	} else if hasSubscript {
		key = arrayKeyWithoutSubscript
		var arrayDestination []any
		if existing, ok := m[key]; !ok {
			arrayDestination = make([]any, index+1)
		} else if existing, ok := existing.([]any); !ok {
			// Error case. Existing isn't of correct type. Can happen if
			// someone is subscripting a field that was previously not an array
			return fmt.Errorf("expected []any at key %v, got %T", key, existing)
		} else if index >= len(existing) {
			// Ensure array is big enough
			arrayDestination = append(existing, make([]any, index-len(existing)+1)...)
		} else {
			arrayDestination = existing
		}

		m[key] = arrayDestination
		if arrayDestination[index] == nil {
			// Doesn't exist case, create the destination.
			// Assumes the destination is a map for now. Theoretically could be
			// extended to support arrays of arrays, but that's not needed yet.
			destination := make(map[string]any)
			arrayDestination[index] = destination
			if err = putNestedValue(destination, rest, v); err != nil {
				return err
			}
		} else if dst, ok := arrayDestination[index].(map[string]any); ok {
			// Already exists case, correct type
			if putNestedValue(dst, rest, v); err != nil {
				return err
			}
		} else {
			// Already exists, incorrect type. Error
			// This shouldn't be possible.
			return fmt.Errorf("expected map at %v[%v], got %T", key, index, arrayDestination[index])
		}

		return nil
	} else if len(rest) == 0 {
		// Base case. Single key. Just set into destination
		m[key] = v
		return nil
	}

	if existing, ok := m[key]; !ok {
		destination := make(map[string]any)
		m[key] = destination
		return putNestedValue(destination, rest, v)
	} else if destination, ok := existing.(map[string]any); ok {
		return putNestedValue(destination, rest, v)
	} else {
		// Error case. Existing isn't of correct type. Can happen if prior comment
		// referred to value as an error
		return fmt.Errorf("expected map[string]any at key %v, got %T", key, existing)
	}
}

// extractArraySubscript extracts the left array subscript from a key of
// the form  `foo[bar][baz]` -> "bar".
// Returns the key without the subscript, the index, and a bool indicating if
// the key had a subscript.
// If the key has a subscript, but the subscript is not a valid integer, returns an error.
//
// This can be adapted to support multidimensional subscripts probably fairly
// easily by retuning a list of ints
func extractArraySubscript(str string) (string, int, bool, error) {
	subscriptIdx := strings.Index(str, "[")
	if subscriptIdx == -1 {
		return "", -1, false, nil
	}

	subscript := strings.Split(str[subscriptIdx+1:], "]")[0]
	if len(subscript) == 0 {
		return "", -1, false, fmt.Errorf("empty subscript not allowed")
	}

	index, err := strconv.Atoi(subscript)
	if err != nil {
		return "", -1, false, fmt.Errorf("expected integer index in key %v", str)
	} else if index < 0 {
		return "", -1, false, fmt.Errorf("subscript '%v' is invalid. index must be positive", subscript)
	}

	return str[:subscriptIdx], index, true, nil
}
