#!/bin/bash
set -e

script_dir="$(dirname "$(readlink -f "$0")")"
chroma_so="$script_dir/chroma-parser.so"
examples_dir="$script_dir/examples"

if [ ! -f "$chroma_so" ]; then
    echo "Error: $chroma_so not found. Please build it first."
    exit 1
fi

if [ ! -d "$examples_dir" ]; then
    echo "Error: Examples directory not found at $examples_dir"
    exit 1
fi

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

echo "Copying examples to $work_dir"
cp -r "$examples_dir/"* "$work_dir"

# Create minimal gtags.conf
cat <<EOF > "$work_dir/gtags.conf"
default:
	:tc=chroma:

chroma:
	:tc=chroma-parser:

chroma-parser:
	:chromalib=$chroma_so:
	:ctagscom=ctags:
	:langmap=cpp\:.cpp.h:
	:langmap=python\:.py:
	:langmap=javascript\:.js:
	:langmap=tcl\:.tcl:
EOF

export GTAGSCONF="$work_dir/gtags.conf"

cd "$work_dir"

# Check if global is available
if ! command -v global &> /dev/null; then
    echo "Error: global command not found in PATH"
    exit 1
fi

# Check if ctags is available
if ! command -v ctags &> /dev/null; then
    echo "Error: ctags command not found in PATH"
    exit 1
fi

echo "Running gtags..."
gtags --verbose

if [ ! -f GTAGS ]; then
    echo "Error: GTAGS not created"
    exit 1
fi

echo "Verifying tags..."

check_tag() {
    tag=$1
    echo "Checking for tag: $tag"
    if global -x "$tag" | grep -q "$tag"; then
        echo "OK: Found $tag"
    else
        echo "FAIL: Did not find $tag"
        exit 1
    fi
}

check_tag "Greeter"
check_tag "Calculator"
check_tag "User"
check_tag "fibonacci"

echo "All tests passed!"
