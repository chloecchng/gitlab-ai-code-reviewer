#!/bin/bash

INPUT_FILE="groups.txt"

OUTPUT_FILE="gitlab_env_vars.txt"

while read PROJECT_ID; do
    echo "processing $PROJECT_ID"
    NAME=$(curl --header "PRIVATE-TOKEN: glpat-rn2dI_6jhgMu_7sQbRrWw286MQp1Omo1M3h2Cw.01.120i6p1r6" "https://gitlab.com/api/v4/projects/${PROJECT_ID}" | jq -r '.name')
    REQ=$(curl --header "PRIVATE-TOKEN: glpat-rn2dI_6jhgMu_7sQbRrWw286MQp1Omo1M3h2Cw.01.120i6p1r6" "https://gitlab.com/api/v4/projects/${PROJECT_ID}/variables" | jq -r --arg proj "$NAME" '.[] | "\($proj), " + .key')
    # echo "$REQ" | jq -r  --arg proj "$PROJECT_ID" '.[] | "\($proj), " + .key ' >> $OUTPUT_FILE
    echo "$REQ" >> $OUTPUT_FILE
done < gitlab_project_list.txt