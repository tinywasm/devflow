# Quickstart JULES API.

## Step 1: List your available sources
First, you need to find the name of the source you want to work with (e.g., your GitHub repo). This command will return a list of all sources you have connected to Jules.
```bash
curl 'https://jules.googleapis.com/v1alpha/sources' \
    -H 'X-Goog-Api-Key: YOUR_API_KEY'
```
The response will look something like this:

```json
{
  "sources": [
    {
      "name": "sources/github/bobalover/boba",
      "id": "github/bobalover/boba",
      "githubRepo": {
        "owner": "bobalover",
        "repo": "boba"
      }
    }
  ],
  "nextPageToken": "github/bobalover/boba-web"
}
```

## Step 2: Create a new session
Now, create a new session. You'll need the source name from the previous step. This request tells Jules to create a boba app in the specified repository.

```bash
curl 'https://jules.googleapis.com/v1alpha/sessions' \
    -X POST \
    -H "Content-Type: application/json" \
    -H 'X-Goog-Api-Key: YOUR_API_KEY' \
    -d '{
      "prompt": "Create a boba app!",
      "sourceContext": {
        "source": "sources/github/bobalover/boba",
        "githubRepoContext": {
          "startingBranch": "main"
        }
      },
      "automationMode": "AUTO_CREATE_PR",
      "title": "Boba App"
    }'
```

The automationMode field is optional. By default, no PR will be automatically created.

The immediate response will look something like this:
```json
{
        "name": "sessions/31415926535897932384",
        "id": "31415926535897932384",
        "title": "Boba App",
        "sourceContext": {
          "source": "sources/github/bobalover/boba",
          "githubRepoContext": {
            "startingBranch": "main"
          }
        },
        "prompt": "Create a boba app!"
      }
```

You can poll the latest session information using GetSession or ListSessions. For example, if a PR was automatically created, you can see the PR in the session output.
```json
{
  "name": "sessions/31415926535897932384",
  "id": "31415926535897932384",
  "title": "Boba App",
  "sourceContext": {
    "source": "sources/github/bobalover/boba",
    "githubRepoContext": {
      "startingBranch": "main"
    }
  },
  "prompt": "Create a boba app!",
  "outputs": [
    {
      "pullRequest": {
        "url": "https://github.com/bobalover/boba/pull/35",
        "title": "Create a boba app",
        "description": "This change adds the initial implementation of a boba app."
      }
    }
  ]
}
```
By default, sessions created through the API will have their plans automatically approved. If you want to create a session that requires explicit plan approval, set the requirePlanApproval field to true.

## Step 3: Listing sessions
You can list your sessions as follows.
```bash
curl 'https://jules.googleapis.com/v1alpha/sessions?pageSize=5' \
    -H 'X-Goog-Api-Key: YOUR_API_KEY'
```
## Step 4: Approve plan
If your session requires explicit plan approval, you can approve the latest plan as follows:

```bash
curl 'https://jules.googleapis.com/v1alpha/sessions/SESSION_ID:approvePlan' \
    -X POST \
    -H "Content-Type: application/json" \
    -H 'X-Goog-Api-Key: YOUR_API_KEY'
```

## Step 5: Activities and interacting with the agent
To list activities in a session:
```bash
curl 'https://jules.googleapis.com/v1alpha/sessions/SESSION_ID/activities?pageSize=30' \
    -H 'X-Goog-Api-Key: YOUR_API_KEY'
```
To send a message to the agent:
```bash
curl 'https://jules.googleapis.com/v1alpha/sessions/SESSION_ID:sendMessage' \
    -X POST \
    -H "Content-Type: application/json" \
    -H 'X-Goog-Api-Key: YOUR_API_KEY' \
    -d '{
      "prompt": "Can you make the app corgi themed?"
    }'
```
The response will be empty because the agent will send its response in the next activity. To see the agent's response, list the activities again.
Here is a consolidated example of a `ListActivities` response:

```json
{
  "activities": [
    {
      "name": "sessions/SESSION_ID/activities/ACT_ID_1",
      "originator": "agent",
      "planGenerated": {
        "plan": {
          "id": "PLAN_ID",
          "steps": [
            { "title": "Setup the environment", "index": 0 },
            { "title": "Modify `src/App.js`", "index": 1 },
            { "title": "Submit the changes", "index": 4 }
          ]
        }
      }
    },
    {
      "name": "sessions/SESSION_ID/activities/ACT_ID_2",
      "originator": "user",
      "planApproved": { "planId": "PLAN_ID" }
    },
    {
      "name": "sessions/SESSION_ID/activities/ACT_ID_3",
      "originator": "agent",
      "progressUpdated": { "title": "Ran bash command", "description": "npm install" },
      "artifacts": [{ "bashOutput": { "command": "npm install", "exitCode": 0 } }]
    },
    {
      "name": "sessions/SESSION_ID/activities/ACT_ID_4",
      "originator": "agent",
      "progressUpdated": { "title": "Modified src/App.js" },
      "artifacts": [{ "changeSet": { "source": "sources/github/repo", "gitPatch": { "baseCommitId": "..." } } }]
    },
    {
      "name": "sessions/SESSION_ID/activities/ACT_ID_5",
      "originator": "agent",
      "sessionCompleted": {}
    }
  ]
}
```