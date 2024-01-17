#!/bin/bash

path="username@192.168.0.240:/Users/<username>/TorProject/Server_Repositories/"
# CSV file containing repo names
repo_csv_file="repo.csv"

# CSV file to store the cloning times
csv_file="cloning_times.csv"

# Read repo names from the CSV file into an array
IFS=$'\n' read -d '' -ra repo_names < "$repo_csv_file"

# Iterate over each repo name
for repo_name in "${repo_names[@]}"
do
  # Run git clone using sshpass and measure the time it takes
  duration=$( (time -p sshpass -p 2774 git clone "$path""$repo_name") 2>&1 | grep real | awk '{print $2}' )

  # Append the cloning time to the CSV file
  echo "$repo_name,$duration" >> "$csv_file"
done