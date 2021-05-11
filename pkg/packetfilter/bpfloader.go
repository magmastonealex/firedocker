package packetfilter

// embed import to bring in bpf_filter.o
import _ "embed"

//go:generate clang-9 -g -O2 -Wall -target bpf -c bpf/filter.c -o bpf_filter.o

//go:embed bpf_filter.o
var bpfFilterContents []byte
