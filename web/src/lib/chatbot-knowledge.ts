export type ChatAudience = "public" | "customer";
export type ChatAction = { label: string; href: string };
export type CustomerChatContext = {
  name?: string;
  planName?: string;
  cardCount?: number;
  publishedCards?: number;
  draftCards?: number;
};
export type ChatAnswer = { answer: string; translationKey?: string; params?: Record<string, string | number>; actions: ChatAction[] };
export type ChatProvider = {
  answer(message: string, audience: ChatAudience, context?: CustomerChatContext): ChatAnswer | null;
};

export const chatShortcuts: Record<ChatAudience, ChatAction[]> = {
  public: [
    { label: "Create Card", href: "/register" }, { label: "Templates", href: "/templates" },
    { label: "Pricing", href: "/pricing" }, { label: "Features", href: "/features" },
    { label: "Contact", href: "/contact" }, { label: "Login", href: "/login" },
  ],
  customer: [
    { label: "Create Card", href: "/dashboard/cards/new" }, { label: "My Cards", href: "/dashboard/cards" },
    { label: "Templates", href: "/dashboard/templates" }, { label: "QR Codes", href: "/dashboard/qr-codes" },
    { label: "NFC", href: "/dashboard/nfc" }, { label: "Billing", href: "/dashboard/billing" },
    { label: "Profile", href: "/dashboard/profile" },
  ],
};

export const quickQuestions = [
  "Create a Digital Card", "How QR Works", "How NFC Works", "Templates",
  "Pricing & Plans", "Billing & Payments", "Account / OTP Help", "Contact Support",
];

type KnowledgeEntry = { id: string; keywords: string[]; answer: string; actions?: ChatAction[] };
const entries: KnowledgeEntry[] = [
  { id:"create-card", keywords:["إنشاء بطاقة","أنشئ بطاقة","بطاقة رقمية","بطاقة جديدة","how do i create a card","how to create a card","how to create card","create a digital card","make a card","create card","new card","digital card","digital business card","what is a digital card","what is digital card"], answer:"A Dbizcard is a mobile-friendly public page for your professional details and links. To create one, register or sign in, choose a template, add your information, customize the design, preview it, then publish. You can update the card later.", actions:[{label:"Create Card",href:"/register"},{label:"Explore Features",href:"/features"}] },
  { id:"publish-share", keywords:["نشر","مشاركة البطاقة","رابط البطاقة","publish","share my card","how to share","share card","card link","copy link","share link"], answer:"After you finish editing, publish the card to make its public page available. You can share its public link, use the QR tools for a published card, or associate the link with compatible NFC hardware. Email sharing is available from the dashboard card share tools when configured.", actions:[{label:"My Cards",href:"/dashboard/cards"},{label:"Create Card",href:"/register"}] },
  { id:"qr", keywords:["رمز الاستجابة","رمز كيو آر","مسح الرمز","qr code","qr sharing","qr scan","scan qr","download qr","share qr","qr"], answer:"A Dbizcard QR code points to the public card URL; it does not encode private contact details. Visitors scan it with a phone camera or QR reader to open the card. In the dashboard, open QR Codes or a card’s sharing tools to create, download or share the available QR code and copy the public link.", actions:[{label:"QR Codes",href:"/dashboard/qr-codes"},{label:"How It Works",href:"/features"}] },
  { id:"nfc", keywords:["الاتصال قريب المدى","المشاركة باللمس","what is nfc","how nfc works","nfc sharing","nfc card","nfc tag","nfc","tap to share"], answer:"NFC means Near Field Communication. A configured NFC card or tag can contain the link to your published Dbizcard. A compatible phone brought close to it may open the public card. Phone, browser and hardware support vary; Dbizcard does not claim universal NFC writing support.", actions:[{label:"NFC",href:"/dashboard/nfc"},{label:"NFC Guide",href:"/features"}] },
  { id:"templates", keywords:["القوالب","قالب","التصميم","الألوان","الخطوط","templates","template","customize colors","customize design","fonts","layout","design"], answer:"Choose a template while creating or editing a card. The card editor provides design controls for supported options such as colors, background, typography, layout and button appearance. Preview your changes before saving or publishing.", actions:[{label:"Explore Templates",href:"/templates"}] },
  { id:"plans", keywords:["الأسعار","الخطط","خطة","ترقية","اشتراك","pricing","plans","plan","personal plan","professional plan","business plan","enterprise","upgrade","current plan"], answer:"The Pricing page lists the plans currently configured for Dbizcard. Plan names, included features and limits can vary by the current catalogue. Signed-in customers can review their current subscription and available plans under Plans & Billing.", actions:[{label:"Pricing",href:"/pricing"},{label:"Plans & Billing",href:"/dashboard/billing"}] },
  { id:"billing", keywords:["الفواتير","الدفع","المدفوعات","إيصال","razorpay","payment","billing","receipt","payment history","failed payment","pending payment","duplicate active plan","duplicate plan","test mode","live mode"], answer:"Plans & Billing shows your current subscription, payment history and available receipts. Checkout uses the configured payment provider; when Razorpay is configured in test mode, the page identifies it as test mode, while live checkout depends on the service configuration. A payment that is pending may need confirmation; failed payments can be retried from their history row. Dbizcard blocks checkout for the same plan when you already have an active subscription to it.", actions:[{label:"Plans & Billing",href:"/dashboard/billing"},{label:"Pricing",href:"/pricing"}] },
  { id:"account-otp", keywords:["رمز التحقق","التسجيل","إنشاء حساب","رمز لم يصل","otp not received","otp","verification code","register","registration","sign up","account help","account"], answer:"To create an account, use Register and follow the verification steps shown there. If an OTP does not arrive, check the address or phone you entered and your spam or message filters, then request another code where the screen allows it. For account-specific issues, contact support.", actions:[{label:"Register",href:"/register"},{label:"Contact Support",href:"/contact"}] },
  { id:"password", keywords:["كلمة المرور","تسجيل الدخول","تسجيل الخروج","الملف الشخصي","forgot password","reset password","password help","can't log in","cannot log in","login problem","logout","profile"], answer:"Use Forgot Password from the login page to request a reset. Update your profile from the customer dashboard Profile page. Use the dashboard’s Logout control to end your signed-in session. If you still cannot access your account, contact support.", actions:[{label:"Login",href:"/login"},{label:"Contact Support",href:"/contact"}] },
  { id:"contacts-analytics", keywords:["جهات الاتصال","التحليلات","المشاهدات","contacts","analytics","card views","qr scans","nfc taps","link clicks","contact interactions","contact exchanges","engagement"], answer:"The dashboard includes Contacts and Analytics areas. Supported card engagement includes card views, QR scans, NFC taps, link clicks and contact exchanges. What you can access may depend on your plan and account permissions.", actions:[{label:"Contacts",href:"/dashboard/contacts"},{label:"Analytics",href:"/dashboard/analytics"}] },
  { id:"card-status", keywords:["حالة البطاقة","بطاقة منشورة","مسودة","card status","published card","draft card","my card status"], answer:"Your Cards page shows each card’s current status, such as draft or published. Open a card to review its details and available actions.", actions:[{label:"My Cards",href:"/dashboard/cards"}] },
  { id:"contact-support", keywords:["الدعم","مساعدة","تواصل","contact support","support","send message","talk to someone","help me","human"], answer:"You can send the Dbizcard team a message through the Contact page. Include a short description of what you were trying to do; do not include passwords, OTPs or payment secrets.", actions:[{label:"Contact Support",href:"/contact"}] },
];

const normalize = (text: string) => text.toLowerCase().trim().replace(/[^\p{L}\p{N}\s&]/gu, " ").replace(/\s+/g, " ");

export class LocalKnowledgeProvider implements ChatProvider {
  answer(message: string, audience: ChatAudience, context?: CustomerChatContext): ChatAnswer | null {
    const query = normalize(message);
    if (!query) return null;
    if (audience === "customer" && context && (/\b(my|current)\s+plan\b/.test(query) || /خطتي|الخطة الحالية|اشتراكي/.test(query))) {
      return { translationKey: context.planName ? "Your current plan is {plan}. You can review its status, features and billing history in Plans & Billing." : undefined, params: context.planName ? {plan: context.planName} : undefined, answer: context.planName ? `Your current plan is ${context.planName}. You can review its status, features and billing history in Plans & Billing.` : "I couldn't find an active plan in the dashboard summary. Check Plans & Billing for the current subscription and available plans.", actions:[{label:"Plans & Billing",href:"/dashboard/billing"}] };
    }
    if (audience === "customer" && context && (/\b(my|how many)\s+cards?\b/.test(query) || /بطاقاتي|عدد البطاقات/.test(query))) {
      const detail = context.cardCount === undefined ? "I couldn't load your card count right now." : `You have ${context.cardCount} card${context.cardCount === 1 ? "" : "s"}${context.publishedCards !== undefined ? `, including ${context.publishedCards} published` : ""}${context.draftCards !== undefined ? ` and ${context.draftCards} draft` : ""}.`;
      const translationKey = context.cardCount === undefined ? "I couldn't load your card count right now. Open My Cards to review their individual status." : `You have {count} card${context.cardCount === 1 ? "" : "s"}${context.publishedCards !== undefined ? ", including {published} published" : ""}${context.draftCards !== undefined ? " and {draft} draft" : ""}. Open My Cards to review their individual status.`;
      return { translationKey, params: {count: context.cardCount ?? 0, published: context.publishedCards ?? 0, draft: context.draftCards ?? 0}, answer: `${detail} Open My Cards to review their individual status.`, actions:[{label:"My Cards",href:"/dashboard/cards"}] };
    }
    let best: KnowledgeEntry | undefined;
    let bestScore = 0;
    for (const entry of entries) {
      const score = entry.keywords.reduce((total, keyword) => {
        const normalizedKeyword = normalize(keyword);
        return total + (query.includes(normalizedKeyword) ? Math.max(2, normalizedKeyword.split(" ").length * 2) : 0);
      }, 0);
      if (score > bestScore) { best = entry; bestScore = score; }
    }
    if (!best || bestScore < 2) return null;
    const actions = (best.actions || []).map(action => ({ ...action }));
    if (audience === "customer") {
      return { answer: best.answer, actions: actions.map(action => action.href === "/register" ? { label:"Create Card",href:"/dashboard/cards/new" } : action) };
    }
    return { answer: best.answer, actions: actions.filter(action => !action.href.startsWith("/dashboard/")) };
  }
}

export const localKnowledgeProvider: ChatProvider = new LocalKnowledgeProvider();
export const fallbackAnswer: ChatAnswer = { answer:"I couldn't find a reliable answer for that.", actions:[{label:"Contact Support",href:"/contact"}] };
