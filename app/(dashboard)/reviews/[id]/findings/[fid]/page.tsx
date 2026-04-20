import FindingDetailPage from "./page-client";

export function generateStaticParams() {
  return [{ id: "_", fid: "_" }];
}

export default function Page() {
  return <FindingDetailPage />;
}
