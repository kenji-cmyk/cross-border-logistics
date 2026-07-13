import { useEffect } from "react";

export function useFormErrorFocus() {
  useEffect(() => {
    const focusError = (event: SubmitEvent) => {
      const form = event.target;
      if (!(form instanceof HTMLFormElement)) return;
      window.requestAnimationFrame(() => form.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus());
    };
    document.addEventListener("submit", focusError, true);
    return () => document.removeEventListener("submit", focusError, true);
  }, []);
}
