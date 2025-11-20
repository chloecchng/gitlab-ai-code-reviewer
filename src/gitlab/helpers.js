/**
 * Filter out files that we shouldn't review (images, locks, deletions)
 */
function filterReviewableFiles(diffs) {
  const IGNORED_EXTENSIONS = [".lock", ".png", ".jpg", ".svg", ".json", ".md"];
  
  return diffs.filter(diff => {
    const path = diff.new_path || diff.old_path;
    if (!path) return false;
    
    // Skip deleted files (nothing to comment on)
    if (diff.deleted_file) return false;

    // Skip ignored extensions
    if (IGNORED_EXTENSIONS.some(ext => path.endsWith(ext))) return false;

    return true;
  });
}

/**
 * Normalize Diff structure because GitLab returns slightly different objects
 * for /changes vs /repository/compare
 */
function formatDiffChanges(diffs) {
  return diffs.map(diff => ({
    diff: diff.diff,
    old_path: diff.old_path,
    new_path: diff.new_path,
    new_file: diff.new_file,
    deleted_file: diff.deleted_file,
    renamed_file: diff.renamed_file,
  }));
}

/**
 * Check if re-review is needed based on approval timing
 */
function computeReReviewStatus(mr, approvals) {
  const approvers = approvals.approved_by ?? [];

  if (approvers.length === 0) {
    return { shouldNotify: false, approverUsernames: [] };
  }

  const mrUpdatedAt = new Date(mr.updated_at);
  let requiresReReview = false;
  const approverUsernames = [];

  approvers.forEach(approval => {
    const approvedAt = new Date(approval.approved_at);
    const username = approval.user?.username;

    if (username) approverUsernames.push(username);
    
    // If MR was updated after approval, notify the approver
    if (mrUpdatedAt > approvedAt) {
      requiresReReview = true;
    }
  });

  return {
    shouldNotify: requiresReReview,
    approverUsernames,
  };
}

/**
 * Build global comment for re-review notification
 */
function buildReReviewNotification(usernames) {
  if (!usernames.length) return "";

  const mentions = usernames.map(u => `@${u}`).join(" ");

  return `
🔔 **Approval Status Changed**

${mentions}

⚠️ **New commits were pushed after your approval.**

Please review the latest changes to ensure everything still looks good before the merge.

---
*This is an automated notification from the AI Code Review Bot.*
  `.trim();
}

module.exports = {
  filterReviewableFiles,
  formatDiffChanges,
  computeReReviewStatus,
  buildReReviewNotification,
};