const { 
  getMRDetails, 
  getMRVersions,
  getCommitComparison,
  getApprovals
} = require("../gitlab/api");
const { 
  filterReviewableFiles,
  formatDiffChanges,
  computeReReviewStatus,
  buildReReviewNotification
} = require("../gitlab/helpers");
const { postInlineComment, postGlobalComment } = require("../gitlab/comments");
const { analyzeWithGemini } = require("../ai/gemini");

/**
 * Orchestrate the review
 */
async function processCodeReview(projectId, mrIid, oldRev, newRev) {
  console.log(`🚀 Starting Review for MR !${mrIid}`);

  // Fetch MR Details and Approvals
  const mrData = await getMRDetails(projectId, mrIid);
  const approvals = await getApprovals(projectId, mrIid);

  // Check if we need to notify previous approvers
  const { shouldNotify, approverUsernames } = computeReReviewStatus(mrData, approvals);
  
  if (shouldNotify && approverUsernames.length > 0) {
    console.log(`🔔 Notifying previous approvers: ${approverUsernames.join(", ")}`);
    const notification = buildReReviewNotification(approverUsernames);
    await postGlobalComment(projectId, mrIid, notification);
  }

  // Fetch MR Versions to decide strategy
  // We need versions to know if this is the very first iteration or a follow-up
  const versions = await getMRVersions(projectId, mrIid);

  let rawDiffs = [];

  // STRATEGY DECISION: FULL vs INCREMENTAL
  
  // Case A: First Version (Fresh MR)
  // Even if oldRev exists (from the push), we want to review the WHOLE file context,
  // not just the diff of the last commit.
  if (versions.length <= 1) {
    console.log(`📝 Strategy: FULL REVIEW (First Version Detected)`);
    rawDiffs = mrData.changes;
  } 
  // Case B: Subsequent Versions (Iterative fixes)
  // We only want to review what changed since the last review.
  else {
    console.log(`📝 Strategy: INCREMENTAL REVIEW (Versions: ${versions.length})`);
    console.log(`🔗 Comparing ${oldRev} -> ${newRev}`);

    if (oldRev && newRev) {
      const comparison = await getCommitComparison(projectId, oldRev, newRev);
      rawDiffs = comparison.diffs || [];
    } else {
      console.warn("⚠️ Missing SHAs for incremental review. Skipping to avoid noise.");
      return;
    }
  }

  // Filter & Format
  const reviewableDiffs = filterReviewableFiles(rawDiffs);
  
  if (reviewableDiffs.length === 0) {
    console.log("✅ No reviewable code changes found.");
    return;
  }

  console.log(`🔍 Analyzing ${reviewableDiffs.length} files...`);
  const formattedDiffs = formatDiffChanges(reviewableDiffs);

  // AI Analysis
  const comments = await analyzeWithGemini(formattedDiffs);
  console.log(`🤖 AI generated ${comments.length} comments.`);

  // Post Comments
  // We already have mrData from earlier
  for (const comment of comments) {
    await postInlineComment(projectId, mrIid, comment, mrData);
    await new Promise(r => setTimeout(r, 500)); // Rate limit protection
  }

  console.log("✅ Review execution finished.");
}

/**
 * Webhook Entry Point
 */
async function processMRWebhook(event) {
  try {
    if (event.object_kind !== "merge_request") return;

    const attrs = event.object_attributes;
    const action = attrs.action;
    const projectId = event.project.id;
    const mrIid = attrs.iid;
    
    // GATEKEEPER: FILTER NON-CODE EVENTS

    // If the MR is not open, ignore it (merged/closed)
    if (attrs.state !== 'opened') {
      console.log(`🚫 Skipping MR !${mrIid} (State: ${attrs.state})`);
      return;
    }

    // CHECK FOR CODE CHANGES
    // 'open' / 'reopen' always implies code needs review.
    // 'update' is the tricky one: it fires for title changes AND code pushes.
    let isCodeChange = false;
    
    if (action === 'open' || action === 'reopen') {
      isCodeChange = true;
    } else if (action === 'update') {

      // Check if oldrev exists. 
      // If I change the Title, GitLab sends oldrev: null (or missing).
      // If I push Code, GitLab sends oldrev: "sha123..."
      if (attrs.oldrev) {
        isCodeChange = true;
      } else {
        console.log(`🚫 Skipping MR !${mrIid} update (Metadata change only, no code push)`);
        return;
      }
    }

    if (!isCodeChange) return;

    // EXECUTE
    
    const oldRev = attrs.oldrev;
    const newRev = attrs.last_commit.id;

    await processCodeReview(projectId, mrIid, oldRev, newRev);

  } catch (err) {
    console.error("❌ Webhook Handler Error:", err.message);
    console.error(err.stack);
  }
}

module.exports = { processMRWebhook };