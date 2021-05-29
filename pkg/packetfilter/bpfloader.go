package packetfilter

// embed import to bring in bpf_filter.o
import _ "embed"

//go:generate bash -c "if [ bpf_filter.o -ot bpf/filter.c ]; then clang -g -O2 -Wall -target bpf -c bpf/filter.c -o bpf_filter.o; fi"
 
//go:embed bpf_filter.o
var bpfFilterContents []byte
