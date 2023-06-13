#!/bin/bash

ORGANIZATION_ID="$1"
GCS_STATEFILE="$2"

TMPFILE=$(mktemp /tmp/XXXXX.tfstate)

gcloud alpha storage cp $GCS_STATEFILE $TMPFILE

# TODO: Handle cases where the ID already is prefixed with ORGANIZATION_ID.
# When using custom roles the ID will already have the ORGANIZATION_ID prefixed.
# TODO: Handle iam_policy resources.
cat $TMPFILE | jq --arg ORG_ID $ORGANIZATION_ID -r '.resources[] | select(.type|contains("iam_binding")) | .instances[] | {member: .attributes.members[], id: .attributes.id | gsub("organizations/"; "") | gsub($ORG_ID + "/"; "") } | ("/organizations/" + $ORG_ID + "/\(.id)/\(.member)") '
cat $TMPFILE | jq --arg ORG_ID $ORGANIZATION_ID -r '.resources[] | select(.type|contains("iam_member")) | .instances[] | {id: .attributes.id | gsub("organizations/"; "") | gsub($ORG_ID + "/"; "")} | ("/organizations/" + $ORG_ID + "/\(.id)")'
