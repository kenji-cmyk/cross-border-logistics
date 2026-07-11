import { LandingPage } from "./pages/LandingPage";
import { TrackingPage } from "./pages/TrackingPage";

export default function App() {
  const match = window.location.pathname.match(/^\/tracking\/([^/]+)$/);
  return match ? <TrackingPage orderId={decodeURIComponent(match[1])} /> : <LandingPage />;
}
