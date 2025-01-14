#!/bin/bash
set -e
shopt -s globstar

# Check that the exported packages are Apache-2.0.
! grep SPDX-License-Identifier pkg/**/**.go | grep -v "// SPDX-License-Identifier: Apache-2.0"

# Check that the exported packages depend only on internal package that are
# licensed Apache-2.0.
deps=$(go list -deps ./pkg/* | grep "github.com/canonical/chisel/internal")
for dep in $deps
do
	folder=$(echo "$dep" | grep -o "internal/.*")
	[ -d "$folder" ]
	! grep SPDX-License-Identifier $folder/**/**.go | grep -v "// SPDX-License-Identifier: Apache-2.0"
done
