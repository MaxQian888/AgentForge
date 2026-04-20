import VCSIntegrationsPage from "./page-client";

export function generateStaticParams() {
  return [{ id: "_" }];
}

export default function Page() {
  return <VCSIntegrationsPage />;
}
