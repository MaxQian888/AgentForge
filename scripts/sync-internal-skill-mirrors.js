#!/usr/bin/env node

/* eslint-disable @typescript-eslint/no-require-imports */

const { syncInternalSkillMirrors } = require("./internal-skill-governance.js");

function main() {
  const result = syncInternalSkillMirrors();
  for (const item of result.updated) {
    console.log(`${item.skillId}: ${item.mirrorTarget}`);
  }
}

if (require.main === module) {
  main();
}

module.exports = {
  main,
};
