declare module "@refinedev/react-router" {
  import type { RouterBindings } from "@refinedev/core";
  import type { FC } from "react";

  const routerProvider: RouterBindings;
  export default routerProvider;

  export const CatchAllNavigate: FC<{ to: string }>;
  export const NavigateToResource: FC<{ resource: string }>;
  export const UnsavedChangesNotifier: FC;
  export const DocumentTitleHandler: FC;
}
