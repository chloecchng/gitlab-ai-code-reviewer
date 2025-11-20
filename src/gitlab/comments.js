const { gitlabApi } = require("./api");

async function postInlineComment(projectId, mrIid, comment, mrData) {
  // Find the file in the current MR state
  // We must use mrData.changes because it contains the latest file paths
  const targetFile = mrData.changes.find(
    c => c.new_path === comment.file
  );

  if (!targetFile) {
    console.log(`⚠️ Skipping comment: File ${comment.file} not found in current MR version.`);
    return;
  }

  // Build the Position Object
  // We use diff_refs from the MR Details (mrData) to ensure we are commenting
  // on the latest version of the MR, regardless of whether we reviewed a diff or the whole thing.
  const position = {
    base_sha: mrData.diff_refs.base_sha,
    start_sha: mrData.diff_refs.start_sha,
    head_sha: mrData.diff_refs.head_sha,
    position_type: "text",
    new_path: comment.file,
    new_line: comment.line,
  };

  console.log(`Posting to ${comment.file}:${comment.line}`);

  try {
    await gitlabApi.post(`/projects/${projectId}/merge_requests/${mrIid}/discussions`, {
      body: `🤖 **AI Review:** ${comment.comment}`,
      position,
    });
  } catch (error) {
    // 400 errors usually mean the AI hallucinated a line number that doesn't exist in the diff
    if (error.response?.status === 400) {
      console.warn(`❌ Failed to post on line ${comment.line} (Line might be unchanged or out of bounds).`);
    } else {
      console.error(`❌ API Error posting comment: ${error.message}`);
    }
  }
}

/**
 * Post a global comment (not attached to any line)
 */
async function postGlobalComment(projectId, mrIid, message) {
  console.log(`📝 Posting global comment to MR !${mrIid}`);
  
  try {
    await gitlabApi.post(`/projects/${projectId}/merge_requests/${mrIid}/notes`, {
      body: message,
    });
    console.log(`✅ Global comment posted successfully`);
  } catch (error) {
    console.error(`❌ Failed to post global comment: ${error.message}`);
  }
}

module.exports = { postInlineComment, postGlobalComment };