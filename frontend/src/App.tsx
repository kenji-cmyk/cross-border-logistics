import { lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes, useParams } from "react-router-dom";
import { AppProviders } from "./app/providers";
import { PageSkeleton } from "./components/ui/Feedback";
import { useRouteFocus } from "./hooks/useRouteFocus";
import { useFormErrorFocus } from "./hooks/useFormErrorFocus";
import { CheckoutPage } from "./pages/CheckoutPage";
import { DepositPaymentPage } from "./pages/DepositPaymentPage";
import { LandingPage } from "./pages/LandingPage";
import { NewQuotationPage } from "./pages/NewQuotationPage";
import { QuotationReviewPage } from "./pages/QuotationReviewPage";
import { RatesPage } from "./pages/RatesPage";
import { NotFoundPage, SystemUnavailablePage } from "./pages/SystemPages";
import { TrackingPage } from "./pages/TrackingPage";
const WarehouseReceivePage = lazy(() => import("./pages/WarehouseReceivePage").then((module) => ({ default: module.WarehouseReceivePage })));
const PackageDetailPage = lazy(() => import("./pages/PackageDetailPage").then((module) => ({ default: module.PackageDetailPage })));
function TrackingRoute() { const { orderId = "" } = useParams(); return <TrackingPage orderId={orderId} />; }
function RouteEffects() { useRouteFocus(); useFormErrorFocus(); return null; }
export default function App() { return <AppProviders><BrowserRouter><RouteEffects /><Suspense fallback={<div className="mx-auto max-w-5xl px-5 py-16"><PageSkeleton /></div>}><Routes><Route path="/" element={<LandingPage />} /><Route path="/quote" element={<NewQuotationPage />} /><Route path="/quote/:quotationId" element={<QuotationReviewPage />} /><Route path="/quote/:quotationId/checkout" element={<CheckoutPage />} /><Route path="/orders/:orderId/payment" element={<DepositPaymentPage />} /><Route path="/tracking/:orderId" element={<TrackingRoute />} /><Route path="/rates" element={<RatesPage />} /><Route path="/warehouse/receive" element={<WarehouseReceivePage />} /><Route path="/warehouse/packages/:packageId" element={<PackageDetailPage />} /><Route path="/admin/rates" element={<RatesPage admin />} /><Route path="/system-unavailable" element={<SystemUnavailablePage />} /><Route path="/404" element={<NotFoundPage />} /><Route path="*" element={<NotFoundPage />} /></Routes></Suspense></BrowserRouter></AppProviders>; }
