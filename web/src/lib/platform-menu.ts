import { BarChart3, Building2, CreditCard, Globe2, LayoutTemplate, Mail, MessageCircle, Palette, QrCode, Share2, SmartphoneNfc, Users, UserRound, type LucideIcon } from "lucide-react";

export type PlatformMenuItem = { title: string; description: string; href: string; icon: LucideIcon };
export type PlatformMenuGroup = { heading: string; items: PlatformMenuItem[] };

export const platformMenuGroups: PlatformMenuGroup[] = [
  { heading: "Explore Plans", items: [
    { title: "Personal", description: "Start sharing your professional identity.", href: "/pricing", icon: UserRound },
    { title: "Professional", description: "Build your personal brand and network.", href: "/pricing", icon: CreditCard },
    { title: "Business", description: "Bring a consistent identity to your team.", href: "/business", icon: Building2 },
    { title: "Enterprise", description: "Explore digital cards for your organization.", href: "/business", icon: Building2 },
  ] },
  { heading: "Create & Share", items: [
    { title: "Digital Business Cards", description: "Create and publish a professional card.", href: "/features#digital-cards", icon: CreditCard },
    { title: "Templates", description: "Choose and customize a card design.", href: "/templates", icon: LayoutTemplate },
    { title: "QR Code Sharing", description: "Let visitors scan to open your public card.", href: "/features#qr", icon: QrCode },
    { title: "NFC Sharing", description: "Link a compatible tag to your digital card.", href: "/features#nfc", icon: SmartphoneNfc },
    { title: "Email Sharing", description: "Send a public card link by email.", href: "/features#email-sharing", icon: Mail },
    { title: "Save Contact", description: "Let visitors save supported public details.", href: "/features#save-contact", icon: UserRound },
  ] },
  { heading: "Connect & Engage", items: [
    { title: "Contact Management", description: "Keep recorded connections organized.", href: "/features#contacts", icon: Users },
    { title: "WhatsApp & Direct Contact", description: "Offer the contact actions on your card.", href: "/features#connect", icon: MessageCircle },
    { title: "Social Profiles", description: "Bring supported social links together.", href: "/features#connect", icon: Share2 },
    { title: "Custom Links", description: "Share your website, portfolio and other links.", href: "/features#connect", icon: Globe2 },
    { title: "Card Sharing", description: "Share with QR, NFC, email or a card link.", href: "/features#qr", icon: Share2 },
  ] },
  { heading: "Manage & Analyze", items: [
    { title: "Analytics & Insights", description: "Review supported views, scans and clicks.", href: "/features#analytics", icon: BarChart3 },
    { title: "Team Management", description: "Manage cards within your plan allowances.", href: "/features#team", icon: Users },
    { title: "Branding", description: "Apply templates, colors and design settings.", href: "/features#branding", icon: Palette },
    { title: "Plans & Billing", description: "Compare plans and included allowances.", href: "/pricing", icon: CreditCard },
    { title: "Business at Scale", description: "Explore consistent cards for teams.", href: "/business", icon: Building2 },
  ] },
];
