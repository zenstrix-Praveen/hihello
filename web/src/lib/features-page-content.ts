// Existing public Features page fallback copy; keep backend reset defaults in sync.
export const featuresPageDefaults = {
  page_heading: "Everything Your Digital Card Needs",
  page_subtitle: "Everything you need to create, share and grow your professional digital identity.",
  enabled: true,
  translations: {} as Record<string, any>,
};

export function readFeaturesPageContent(result: Record<string, unknown>) {
  const content = (result.content ?? result) as Partial<typeof featuresPageDefaults> & {translations?:Record<string,any>};
  return {
    page_heading: typeof content.page_heading === "string" ? content.page_heading : featuresPageDefaults.page_heading,
    page_subtitle: typeof content.page_subtitle === "string" ? content.page_subtitle : featuresPageDefaults.page_subtitle,
    enabled: typeof content.enabled === "boolean" ? content.enabled : featuresPageDefaults.enabled,
    translations: content.translations ?? {},
  };
}
