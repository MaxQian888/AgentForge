import type { Locale } from "./config";
import enMessages from "@/messages/en";
import zhCNMessages from "@/messages/zh-CN";

export const messageBundles: Record<Locale, Record<string, unknown>> = {
  "zh-CN": zhCNMessages,
  en: enMessages,
};
