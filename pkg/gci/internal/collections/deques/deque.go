// Package deques provides queue implementations that automatically evict
// old entries based on a length limit or an age limit.
//
// Two variants are provided:
//
//   - [Counting] caps the number of entries.
//   - [Aging] caps the age of entries.
package deques
