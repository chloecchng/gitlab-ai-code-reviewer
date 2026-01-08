const axios = require("axios");
const { GITLAB_TOKEN, GITLAB_URL } = require("../config");

const gitlabApi = axios.create({
  baseURL: `${GITLAB_URL}/api/v4`,
  headers: { "PRIVATE-TOKEN": GITLAB_TOKEN },
});

/**
 * Get full MR details (including changes and diff_refs)
 */
async function getMRDetails(projectId, mrIid) {
  const { data } = await gitlabApi.get(`/projects/${projectId}/merge_requests/${mrIid}/changes`);
  return data;
}

/**
 * Get all versions of an MR to decide between Full vs Incremental review
 */
async function getMRVersions(projectId, mrIid) {
  const { data } = await gitlabApi.get(`/projects/${projectId}/merge_requests/${mrIid}/versions`);
  return data;
}

/**
 * Get current Approvals state
 */
async function getApprovals(projectId, mrIid) {
  const { data } = await gitlabApi.get(`/projects/${projectId}/merge_requests/${mrIid}/approvals`);
  return data;
}

/**
 * Compare two commits (for Incremental review)
 */
async function getCommitComparison(projectId, fromSha, toSha) {
  const { data } = await gitlabApi.get(`/projects/${projectId}/repository/compare`, {
    params: { from: fromSha, to: toSha }
  });
  return data;
}

module.exports = {
  gitlabApi,
  getMRDetails,
  getMRVersions,
  getApprovals,
  getCommitComparison,
};