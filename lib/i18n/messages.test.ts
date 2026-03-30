import enMessages from "@/messages/en";
import zhCNMessages from "@/messages/zh-CN";
import { messageBundles } from "./messages";

describe("messageBundles", () => {
  it("maps each supported locale to the imported message bundle", () => {
    expect(messageBundles).toMatchObject({
      en: enMessages,
      "zh-CN": zhCNMessages,
    });
  });
});
