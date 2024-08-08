#!/usr/bin/python3
import re

def find_pattern(file_path, pattern):
    """Finds occurrences of a pattern in a file.

    Args:
    file_path: Path to the file.
    pattern: Regular expression pattern to search for.

    Returns:
    A list of matches.
    """
    try:
        with open(file_path, 'r') as file:
            text = file.read()
            matches = re.findall(pattern, text)
            return matches
    except FileNotFoundError as e:
        print(f"{e}")
    except PermissionError as e:
        print(f"{e}")
    except TypeError as e:
        print(f"{e}")
    except Exception as e:
        print(f"{e}")
  

if __name__ == '__main__':
    pods_file_path = 'testcase_pods.log'
    curl_output_file_path = 'testcase_curl_output.log'
    pattern = r'\breviews[a-z0-9\w-]+\b'
    matched_pods = sorted(find_pattern(pods_file_path, pattern))
    matched_curl_output = find_pattern(curl_output_file_path, pattern)
    matched_curl_output_unique = sorted(list(set(matched_curl_output)))
    for pod in matched_pods:
        count = matched_curl_output.count(pod)
        print(f"{pod}: {count}")
    if matched_pods == matched_curl_output_unique:
        print(matched_pods)
        print(matched_curl_output_unique)
        print('\033[92m' + "PASSED!" + '\033[0m')
        exit(0)
    else:
        print(matched_pods)
        print(matched_curl_output_unique)
        print('\033[91m' + "FAILED!" + '\033[0m')
        exit(1)