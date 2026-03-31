import enRoles from "@/messages/en/roles.json";
import zhCNRoles from "@/messages/zh-CN/roles.json";

describe("roles locale messages", () => {
  it("keeps the skill catalog workspace copy available in zh-CN", () => {
    expect(enRoles.workspace).toEqual(
      expect.objectContaining({
        availableSkillsTitle: expect.any(String),
        availableSkillsDesc: expect.any(String),
        availableSkillsEmpty: expect.any(String),
        skillResolvedDetail: expect.any(String),
        skillUnresolved: expect.any(String),
        skillProvenanceExplicit: expect.any(String),
        skillProvenanceTemplate: expect.any(String),
        skillProvenanceInherited: expect.any(String),
        skillPartsLabel: expect.any(String),
        skillPartAgents: expect.any(String),
        skillPartReferences: expect.any(String),
        skillPartScripts: expect.any(String),
        skillPartAssets: expect.any(String),
      })
    );

    expect(zhCNRoles.workspace).toEqual(
      expect.objectContaining({
        availableSkillsTitle: expect.any(String),
        availableSkillsDesc: expect.any(String),
        availableSkillsEmpty: expect.any(String),
        skillResolvedDetail: expect.any(String),
        skillUnresolved: expect.any(String),
        skillProvenanceExplicit: expect.any(String),
        skillProvenanceTemplate: expect.any(String),
        skillProvenanceInherited: expect.any(String),
        skillPartsLabel: expect.any(String),
        skillPartAgents: expect.any(String),
        skillPartReferences: expect.any(String),
        skillPartScripts: expect.any(String),
        skillPartAssets: expect.any(String),
      })
    );
  });

  it("keeps context rail skill resolution copy available in both locales", () => {
    expect(enRoles.contextRail).toEqual(
      expect.objectContaining({
        skillResolution: expect.any(String),
        skillResolutionDesc: expect.any(String),
        skillResolutionNone: expect.any(String),
      })
    );

    expect(zhCNRoles.contextRail).toEqual(
      expect.objectContaining({
        skillResolution: expect.any(String),
        skillResolutionDesc: expect.any(String),
        skillResolutionNone: expect.any(String),
      })
    );
  });
});
