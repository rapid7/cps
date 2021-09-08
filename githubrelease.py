from github import Github
import sys

repo_name = sys.argv[1]
access_token = sys.argv[2]
version = sys.argv[3]
asset_path = sys.argv[4]
asset_name = sys.argv[5]
message = "Release for " + version
g = Github(access_token)
repo = g.get_repo(repo_name)
print(repo)
repo.create_git_release(tag=version, name=version, message=message)
release = repo.get_release(id=version)
release.upload_asset(asset_path, label=asset_name, name=asset_name)