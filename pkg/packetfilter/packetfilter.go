// Package packetfilter provides capabilities of filtering traffic on a particular interface
// to ensure it only ingresses traffic from specific IPs or MACs.
// This is particularly helpful with Firecracker VMs, because it allows filtering the TAP interface
// exposed to the VM to prevent it from spoofing packets or pretending to have a different IP or MAC
// from one it was assigned.
// Filtering is implemented using TC eBPF. The eBPF program expects a map defined for allowed IPs and allowed MACs per ifindex.
// Helper functions are provided to install the eBPF filter, remove the eBPF filter, and add and remove entries
// for IP and MAC whitelisting.
package packetfilter
