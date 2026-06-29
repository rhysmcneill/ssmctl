#!/bin/bash -eu

# build project
# e.g.
# ./autogen.sh
# ./configure
# make -j$(nproc) all

# build fuzzers
# e.g.
# $CXX $CXXFLAGS -std=c++11 -Iinclude \
#     /path/to/name_of_fuzzer.cc -o $OUT/name_of_fuzzer \
#     $LIB_FUZZING_ENGINE /path/to/library.a
#
cd $SRC/ssmctl

go mod download

count=0

# Use process substitution to avoid the subshell variable loss
while read -r line; do
    # Extract just the function name (e.g., FuzzSanitizeBasename)
    func_name=$(echo "$line" | sed -E 's/.*func (Fuzz[A-Za-z0-9_]+)\(.*/\1/')

    # Convert the function name to lowercase for the binary output name
    binary_name=$(echo "$func_name" | tr '[:upper:]' '[:lower:]')

    echo "Building dynamic Go fuzzer: $func_name -> $binary_name"

    # Run the compilation command using the dynamic variables
    compile_native_go_fuzzer ./internal/ssm "$func_name" "$binary_name"

    count=$((count + 1))
done < <(find ./internal/ssm -name "*_fuzz_test.go" -type f | xargs grep -E '^func Fuzz[A-Za-z0-9_]+\(')

# Fixed spacing issues here
if [ "$count" -eq 0 ]; then
  echo "Error: no Fuzz functions found" >&2
  exit 1
fi
