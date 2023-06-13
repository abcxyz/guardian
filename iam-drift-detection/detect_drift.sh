#!/bin/bash
ORGANIZATION_ID=$1
BUCKET_QUERY="labels:terraform"

TMPFILE_TF=$(mktemp /tmp/XXXXX.txt)
TMPFILE_TF_NOIGNORE=$(mktemp /tmp/XXXXX.txt)
TMPFILE_ACTUAL=$(mktemp /tmp/XXXXX.txt)
TMPFILE_ACTUAL_NOIGNORE=$(mktemp /tmp/XXXXX.txt)
TMPFILE_DIFF=$(mktemp /tmp/XXXXX.txt)
TMPFILE_DRIFTIGNORE=$(mktemp /tmp/XXXXX.txt)

ALL_TFSTATE_GCS_URIS=()
BUCKETS=($(gcloud asset search-all-resources \
    --asset-types=storage.googleapis.com/Bucket --query="$BUCKET_QUERY" --read-mask=name \
    "--scope=organizations/$ORGANIZATION_ID" --format=json | jq -r '.[] | (.name|= gsub("//storage.googleapis.com/"; "")) | .name'))
for bucket in "${BUCKETS[@]}"; do
    gcs_uris=($(gcloud storage ls -e "gs://${bucket}/**/default.tfstate"))
    ALL_TFSTATE_GCS_URIS=("${ALL_TFSTATE_GCS_URIS[@]}" "${gcs_uris[@]}")
done

for gcs_uri in "${ALL_TFSTATE_GCS_URIS[@]}"; do
    ./parse_iam_from_tf_state_file.sh  "$ORGANIZATION_ID" "$gcs_uri" >> "$TMPFILE_TF"
done
./get_iam.sh "$ORGANIZATION_ID" > "$TMPFILE_ACTUAL"
cat ".driftignore" > "$TMPFILE_DRIFTIGNORE"

# `comm` requires sorted inputs and this also ensures the output is sorted.
sort -o "$TMPFILE_TF" "$TMPFILE_TF"
sort -o "$TMPFILE_ACTUAL" "$TMPFILE_ACTUAL"
sort -o "$TMPFILE_DRIFTIGNORE" "$TMPFILE_DRIFTIGNORE"

# Get only the resources that aren't in the ".driftignore" file.
comm -2 -3 $TMPFILE_TF $TMPFILE_DRIFTIGNORE > "$TMPFILE_TF_NOIGNORE"
comm -2 -3 $TMPFILE_ACTUAL $TMPFILE_DRIFTIGNORE > "$TMPFILE_ACTUAL_NOIGNORE"

# Compute the diff between the actual IAM and the Terraform IAM.
diff -w "$TMPFILE_TF_NOIGNORE" "$TMPFILE_ACTUAL_NOIGNORE" > "$TMPFILE_DIFF" || true
MISSING=$(grep "^< " "$TMPFILE_DIFF" | sed -E 's/</>/g' || true)
UNEXPECTED=$(grep "^> " "$TMPFILE_DIFF" || true)

if [[ $MISSING != "" ]]; then
    echo "Managed by terraform but missing:"
    echo "$MISSING"
fi
if [[ $UNEXPECTED != "" ]]; then
    echo  "Applied manually by users (Click Ops):"
    echo "$UNEXPECTED"
fi
