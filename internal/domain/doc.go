// Package domain holds the entity row types and the validation and
// normalization rules shared by every entity package.
//
// Row types live here so any package can reference them without import
// cycles; requests, projections, envelopes, and services stay in the entity
// packages, which re-export these rows under aliases.
package domain
