import common from "./common.json";
import auth from "./auth.json";
import dashboard from "./dashboard.json";
import projects from "./projects.json";
import agents from "./agents.json";
import tasks from "./tasks.json";
import teams from "./teams.json";
import sprints from "./sprints.json";
import reviews from "./reviews.json";
import settings from "./settings.json";
import plugins from "./plugins.json";
import cost from "./cost.json";
import scheduler from "./scheduler.json";
import memory from "./memory.json";
import docs from "./docs.json";
import im from "./im.json";
import roles from "./roles.json";
import workflow from "./workflow.json";
import forms from "./forms.json";
import logs from "./logs.json";
import documents from "./documents.json";
import audit from "./audit.json";
import projectTemplates from "./project-templates.json";
import invitations from "./invitations.json";
import { normalizeMessageBundle } from "../normalize";

export default normalizeMessageBundle({
  common,
  auth,
  dashboard,
  projects,
  agents,
  tasks,
  teams,
  sprints,
  reviews,
  settings,
  plugins,
  cost,
  scheduler,
  memory,
  docs,
  im,
  roles,
  workflow,
  forms,
  logs,
  documents,
  audit,
  projectTemplates,
  invitations,
});
