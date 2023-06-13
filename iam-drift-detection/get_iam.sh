# retrieves org and project iam policy and turns them into the format of
# ORG_ID/role:members or ORG_ID/organizations/PROJECT_ID/role:members

ORGANIZATION_ID="$1"

gcloud organizations get-iam-policy $ORGANIZATION_ID --format=json | \
    jq -r --arg ORG_ID $ORGANIZATION_ID '.bindings[] | (.role |= gsub("organizations/"; "")) | (.role|= gsub($ORG_ID + "/"; "")) | ("/organizations/" + $ORG_ID + "/" + .role + "/" + .members[])'

FOLDERS=$(gcloud beta asset search-all-resources \
    --asset-types=cloudresourcemanager.googleapis.com/Folder \
    "--scope=organizations/${ORGANIZATION_ID}" --format=json)

FOLDER_IDS=($(echo $FOLDERS | jq -r '.[] | .name' | sed -E 's/\/\/cloudresourcemanager\.googleapis\.com\/folders\///g'))

for FOLDER in "${FOLDER_IDS[@]}"
do
    gcloud resource-manager folders get-iam-policy $FOLDER --format=json | \
        jq -r --arg ORG_ID $ORGANIZATION_ID --arg FOLDER $FOLDER '.bindings[] | (.role |= gsub("organizations/"; "")) | (.role|= gsub($ORG_ID + "/"; "")) | ("/organizations/" + $ORG_ID + "/folders/" + $FOLDER + "/" + .role + "/" + .members[])'
done

PROJECTS=$(gcloud beta asset search-all-resources \
    --asset-types=cloudresourcemanager.googleapis.com/Project \
    "--scope=organizations/${ORGANIZATION_ID}" --format=json)

PROJECT_NUMS=($(echo $PROJECTS | jq -r '.[] | .project' | sed -E 's/projects\///g'))
PROJECT_IDS=($(echo $PROJECTS | jq -r '.[] | .name' | sed -E 's/\/\/cloudresourcemanager\.googleapis\.com\/projects\///g'))

count="${#PROJECT_NUMS[@]}"
for i in `seq 1 $count`
do
    PROJECT_NUM="${PROJECT_NUMS[$i-1]}"
    PROJECT_ID="${PROJECT_IDS[$i-1]}"
    gcloud projects get-iam-policy $PROJECT_NUM --format=json | \
        jq -r --arg ORG_ID $ORGANIZATION_ID --arg PROJECT $PROJECT_ID '.bindings[] | (.role |= gsub("organizations/"; "")) | (.role|= gsub($ORG_ID + "/"; "")) | ("/organizations/" + $ORG_ID + "/projects/" + $PROJECT + "/" + .role + "/" + .members[])'
done
