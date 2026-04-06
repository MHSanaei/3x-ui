package core

// Annotation is a concept in Xray. This struct is only for documentation. It is not used anywhere.
// Annotations begin with "xray:" in comment, as metadata of functions or types.
type Annotation struct {
	// API is for types or functions that can be used in other libs. Possible values are:
	//
	// * xray:api:beta for types or functions that are ready for use, but maybe changed in the future.
	// * xray:api:stable for types or functions with guarantee of backward compatibility.
	// * xray:api:deprecated for types or functions that should not be used anymore.
	//
	// Types or functions without api annotation should not be used externally.
	API string
}
