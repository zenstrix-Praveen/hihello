import "./globals.css";
import { CustomerLocaleBoundary } from "@/lib/customer-locale";
export const metadata = {
  title: "DBIZCARD | Digital Business Cards",
  applicationName: "DBIZCARD",
  description: "Create, share and manage your DBIZCARD digital business cards.",
  openGraph: {
    siteName: "DBIZCARD",
    title: "DBIZCARD | Digital Business Cards",
    description: "Create, share and manage your DBIZCARD digital business cards.",
  },
  icons: {
    icon: [{ url: "/brand/dbizcard-icon.png", type: "image/png", sizes: "620x620" }],
    shortcut: "/brand/dbizcard-icon.png",
    apple: "/brand/dbizcard-icon.png",
  },
};
export default function RootLayout({children}:{children:React.ReactNode}){ return <html lang="en"><body><CustomerLocaleBoundary>{children}</CustomerLocaleBoundary></body></html> }
