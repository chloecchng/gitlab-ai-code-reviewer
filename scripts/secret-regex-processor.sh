#!/bin/bash

# Takes in unprocessed trace file 
INPUT_FILE=$1

# Define the regexes to prevent TruffleHog false positives
regex_patterns=(
    's/(?i:gitlab|)(?:.|[\n\r]){0,40}?\b([a-zA-Z0-9\-=_]{20,22})\b' # Replaces gitlab tokens (For more information regarding TruffleHog Gitlab regex: https://github.dev/trufflesecurity/trufflehog/blob/main/pkg/detectors/gitlab/v1/gitlab.go#L36)
)

processed=$(grep -v "[[:space:]]$" $INPUT_FILE | grep -v "0K$")

# Apply each regex in array
for regex in ${regex_patterns[@]}; do
    processed=$(cat "$processed" | perl -pe "s/$regex/XXXXX/g")
done

echo "$processed"