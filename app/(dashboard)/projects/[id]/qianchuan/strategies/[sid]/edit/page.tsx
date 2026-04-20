import QianchuanStrategyEditPage from "./page-client";

export function generateStaticParams() {
  return [{ id: "_", sid: "_" }];
}

export default function Page() {
  return <QianchuanStrategyEditPage />;
}
