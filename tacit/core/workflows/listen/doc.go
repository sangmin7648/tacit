// Package listen is the daemon: it hears the microphone and turns speech into
// notes, keeps the PID file that tells everyone it is running, and reports
// what it is doing through the event log.
//
// Its stages are components; this package alone knows their order. See
// docs/concepts/pipeline.md.
package listen
