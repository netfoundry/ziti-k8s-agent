#!/usr/bin/python3
"""
Verification Script for Ziti K8s Agent Tests

This script verifies the results of the Bookinfo application test deployment across AWS and GKE clusters. A passing
result is at least one request per reviews pod.

Usage:
    verify_test_results.py <file_path> [file_paths...]

Exit codes:
- 0: Success - all conditions met
- 1: Failure - missing pods or reviews
- 2: Invalid usage
"""

import re
import sys


def find_pattern(file_path, pattern):
    """
    Search for regex pattern matches in a file and return alnum ascending unique matches.

    Args:
        file_path (str): Path to the file to search
        pattern (str): Regular expression pattern to search for

    Returns:
        set: Sorted unique matches found in the file

    Raises:
        FileNotFoundError: If the specified file doesn't exist
        Exception: For other unexpected errors during file processing
    """
    matches = set()
    try:
        with open(file_path, 'r') as file:
            for line in file:
                found = re.findall(pattern, line)
                matches.update(found)
        return sorted(matches)
    except FileNotFoundError as e:
        print(f"Error: Could not find file {file_path}")
        print(f"{e}")
    except Exception as e:
        print(f"Error processing file: {e}")


if __name__ == '__main__':
    if len(sys.argv) != 3:
        print("Error: Exactly two file paths are required")
        print(f"Usage: {sys.argv[0]} <pods_file_path> <curl_output_file_path>")
        exit(2)

    pods_file_path = sys.argv[1]
    curl_output_file_path = sys.argv[2]

    # Find matches of reviews pod names
    pattern = r'reviews-v[123]'

    matched_pods = find_pattern(pods_file_path, pattern)
    matched_reviews = find_pattern(curl_output_file_path, pattern)

    # Test passes if the matches are identical: the list of running pods and the list of pods that logged at least one
    # request
    if matched_pods == matched_reviews:
        print('\033[92m' + "SUCCESS!" + '\033[0m')
        exit(0)
    else:
        print('\033[91m' + "FAILURE!" + '\033[0m')
        print(f"Running pods: {matched_pods}")
        print(f"Pods that logged requests: {matched_reviews}")
        exit(1)
