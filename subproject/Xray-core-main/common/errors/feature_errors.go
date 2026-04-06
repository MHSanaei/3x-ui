package errors

import (
	"context"
)

// PrintNonRemovalDeprecatedFeatureWarning prints a warning of the deprecated feature that won't be removed in the near future.
// Do not remove this function even there is no reference to it.
func PrintNonRemovalDeprecatedFeatureWarning(sourceFeature string, targetFeature string) {
	LogWarning(context.Background(), "The feature "+sourceFeature+" is deprecated, not recommended for using and might be removed. Please migrate to "+targetFeature+" as soon as possible.")
}

// PrintDeprecatedFeatureWarning prints a warning for deprecated and going to be removed feature.
// Do not remove this function even there is no reference to it.
func PrintDeprecatedFeatureWarning(feature string, migrateFeature string) {
	if len(migrateFeature) > 0 {
		LogWarning(context.Background(), "This feature "+feature+" is deprecated, will be removed soon and being migrated to "+migrateFeature+". Please update your config(s) according to release note and documentation before removal.")
	} else {
		LogWarning(context.Background(), "This feature "+feature+" is deprecated and will be removed soon. Please update your config(s) according to release note and documentation before removal.")
	}
}

// PrintRemovedFeatureError prints an error message for removed feature then return an error. And after long enough time the message can also be removed, uses as an indicator.
// Do not remove this function even there is no reference to it.
func PrintRemovedFeatureError(feature string, migrateFeature string) error {
	if len(migrateFeature) > 0 {
		return New("The feature " + feature + " has been removed and migrated to " + migrateFeature + ". Please update your config(s) according to release note and documentation.")
	} else {
		return New("The feature " + feature + " has been removed. Please update your config(s) according to release note and documentation.")
	}
}
