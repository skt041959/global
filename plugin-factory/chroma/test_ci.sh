#!/bin/bash
set -e

# Setup paths
PLUGIN_DIR=$(pwd)/plugin-factory/chroma
SO_PATH=$PLUGIN_DIR/chroma-parser.so
CONF_IN=gtags.conf.in
CONF=gtags.conf.ci

if [ ! -f "$SO_PATH" ]; then
    echo "Plugin not built!"
    exit 1
fi

# Create test directory
mkdir -p test_chroma
cd test_chroma

# 1. Create gtags.conf.ci
# We copy from ../gtags.conf.in and modify variables
cp ../$CONF_IN $CONF

# Replace @EXUBERANT_CTAGS@ with system ctags (usually /usr/bin/ctags or check 'type -P ctags')
CTAGS_BIN=$(type -P ctags || echo "/usr/bin/ctags")
sed -i "s|@EXUBERANT_CTAGS@|$CTAGS_BIN|g" $CONF
sed -i "s|@UNIVERSAL_CTAGS@|$CTAGS_BIN|g" $CONF

# Replace chromalib path
# Note: we need to escape $ and /
ESCAPED_SO_PATH=$(echo $SO_PATH | sed 's/\//\\\//g')
sed -i "s|\\\$libdir\/gtags\/chroma-parser.so|$ESCAPED_SO_PATH|g" $CONF

# Remove unresolved variables and ensure proper defaults for test
sed -i 's|@DEFAULTSKIP@||g' $CONF
sed -i 's|@DEFAULTLANGMAP@|c:.c.h|g' $CONF
sed -i 's|@DEFAULTLANGMAP_QUOTED@|c\\\:.c.h|g' $CONF
sed -i 's|@DEFAULTINCLUDEFILESUFFIXES@||g' $CONF
sed -i 's|@POSIX_SORT@|sort|g' $CONF


# 2. Generate Source Files

# C++
cat <<EOF > test.cpp
#include <iostream>

class Greeter {
public:
    void greet() {
        std::cout << "Hello C++" << std::endl;
    }
};

int main() {
    Greeter g;
    g.greet();
    return 0;
}
EOF

# Python
cat <<EOF > test.py
def hello_python():
    print("Hello Python")

class PyGreeter:
    def greet(self):
        hello_python()

if __name__ == "__main__":
    pg = PyGreeter()
    pg.greet()
EOF

# JavaScript
cat <<EOF > test.js
function helloJS() {
    console.log("Hello JS");
}

class JSGreeter {
    greet() {
        helloJS();
    }
}

const jsg = new JSGreeter();
jsg.greet();
EOF

# Tcl
cat <<EOF > test.tcl
proc hello_tcl {} {
    puts "Hello Tcl"
}

hello_tcl
EOF

# 3. Run gtags
echo "Running gtags with chroma plugin..."
gtags --gtagsconf=$CONF --gtagslabel=chroma -v

# 4. Verify Tags

# Helper function to verify a tag
verify_tag() {
    TAG=$1
    FILE=$2
    RESULT=$(global -x $TAG --gtagsconf=$CONF --gtagslabel=chroma)
    if [[ "$RESULT" == *"$TAG"* && "$RESULT" == *"$FILE"* ]]; then
        echo "PASS: Tag '$TAG' found in '$FILE'"
    else
        echo "FAIL: Tag '$TAG' NOT found in '$FILE'"
        echo "Result was: $RESULT"
        exit 1
    fi
}

# Verify definitions (via ctags integration)
verify_tag "Greeter" "test.cpp"
verify_tag "hello_python" "test.py"
verify_tag "helloJS" "test.js"
verify_tag "hello_tcl" "test.tcl"

# Verify references (via chroma integration)
# Note: To verify references specifically coming from chroma, we check for usages.
# 'greet' is used in main() in test.cpp
# 'hello_python' is used in greet() in test.py
# 'helloJS' is used in greet() in test.js
# 'hello_tcl' is used in main script in test.tcl

# global -r finds references
echo "Checking references..."

check_reference() {
    TAG=$1
    FILE=$2
    RESULT=$(global -r $TAG --gtagsconf=$CONF --gtagslabel=chroma)
    if [[ "$RESULT" == *"$FILE"* ]]; then
        echo "PASS: Reference to '$TAG' found in '$FILE'"
    else
        # Note: If ctags also parses references or if chroma fails, this might fail.
        # But our plugin merges them.
        echo "FAIL: Reference to '$TAG' NOT found in '$FILE'"
        echo "Result was: $RESULT"
        # Not exiting immediately as reference parsing might be tricky depending on lexer
        # but for the task "compile chroma... implement API... generate reference", it should work.
        exit 1
    fi
}

check_reference "Greeter" "test.cpp"
check_reference "hello_python" "test.py"
check_reference "helloJS" "test.js"
check_reference "hello_tcl" "test.tcl"

echo "All tests passed!"

# Cleanup
cd ..
rm -rf test_chroma
