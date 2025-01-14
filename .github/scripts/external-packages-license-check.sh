#!/bin/bash
set -e
shopt -s globstar

check_file() {
	file=$1
	license=$(grep SPDX-License-Identifier $file || true)

	# Empty license or different from Apache-2.0.
	if [ -z "$license" ] || [ -n "$(echo $license | grep -v '// SPDX-License-Identifier: Apache-2.0')" ]
	then
		echo "wrong license in file:" $file
		exit 1
	fi
}

# Check that the exported packages are Apache-2.0.
files=pkg/**/**.go
for file in $files
do
	check_file $file
done

# Check that the exported packages depend only on internal package that are
# licensed Apache-2.0.
deps=$(go list -deps -test ./pkg/* | grep "github.com/canonical/chisel/internal")
for dep in $deps
do
	folder=$(echo "$dep" | grep -o "internal/.*")
	[ -d "$folder" ]
	files=$folder/**/**.go
	for file in $files
	do
		check_file $file
	done
done
