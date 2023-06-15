#!/bin/sh
# Copyright 2023 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


ORGANIZATION_ID="$1"
GCS_STATEFILE="$2"

TMPFILE=$(mktemp /tmp/XXXXX.tfstate)

gcloud storage cp $GCS_STATEFILE $TMPFILE

# TODO: Handle cases where the ID already is prefixed with ORGANIZATION_ID.
# When using custom roles the ID will already have the ORGANIZATION_ID prefixed.
# TODO: Handle iam_policy resources.
cat $TMPFILE | jq --arg ORG_ID $ORGANIZATION_ID -r '.resources[] | select(.type|contains("iam_binding")) | .instances[] | {member: .attributes.members[], id: .attributes.id | gsub("organizations/"; "") | gsub($ORG_ID + "/"; "") } | ("/organizations/" + $ORG_ID + "/\(.id)/\(.member)") '
cat $TMPFILE | jq --arg ORG_ID $ORGANIZATION_ID -r '.resources[] | select(.type|contains("iam_member")) | .instances[] | {id: .attributes.id | gsub("organizations/"; "") | gsub($ORG_ID + "/"; "")} | ("/organizations/" + $ORG_ID + "/\(.id)")'
