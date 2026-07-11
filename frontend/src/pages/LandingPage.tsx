import { BackgroundVideo } from "../components/landing/BackgroundVideo";
import { HeroSection } from "../components/landing/HeroSection";
import { LogisticsRouteGraphic } from "../components/landing/LogisticsRouteGraphic";
import { Navbar } from "../components/landing/Navbar";

export function LandingPage() { return <div className="relative min-h-screen w-full overflow-hidden bg-[#F7F9FC]"><BackgroundVideo /><LogisticsRouteGraphic /><Navbar /><HeroSection /></div>; }
