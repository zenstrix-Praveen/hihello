import {
  BarChart3,
  ContactRound,
  Mail,
  MapPin,
  MessageCircle,
  Nfc,
  Phone,
  QrCode,
  Share2,
} from "lucide-react";

const leftFeatures = [
  {
    title: "Click to Call",
    description: "Visitors can call you instantly from your digital business card.",
    icon: Phone,
  },
  {
    title: "WhatsApp",
    description: "Start a WhatsApp conversation without manually saving the number.",
    icon: MessageCircle,
  },
  {
    title: "Email",
    description: "Let visitors send you an email directly from your card.",
    icon: Mail,
  },
  {
    title: "QR Sharing",
    description: "Share your digital card instantly using a scannable QR code.",
    icon: QrCode,
  },
];

const rightFeatures = [
  {
    title: "Click to Map",
    description: "Open your business location directly in maps for easy navigation.",
    icon: MapPin,
  },
  {
    title: "Save Contact",
    description: "Visitors can save your contact information directly to their phone.",
    icon: ContactRound,
  },
  {
    title: "NFC Sharing",
    description: "Share your digital card instantly with compatible NFC devices.",
    icon: Nfc,
  },
  {
    title: "Analytics",
    description: "Track card views, QR scans, NFC taps and engagement.",
    icon: BarChart3,
  },
];

function FeatureItem({
  title,
  description,
  icon: Icon,
  reverse = false,
}: {
  title: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
  reverse?: boolean;
}) {
  return (
    <div
      className={`flex items-center gap-4 ${
        reverse ? "lg:flex-row-reverse lg:text-left" : "lg:text-right"
      }`}
    >
      <div className="flex-1">
        <h3 className="text-xl font-semibold text-slate-900">{title}</h3>
        <p className="mt-2 leading-7 text-slate-500">{description}</p>
      </div>

      <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-full bg-blue-600 text-white shadow-lg">
        <Icon className="h-7 w-7" />
      </div>
    </div>
  );
}

export default function FeatureShowcase() {
  return (
    <section className="bg-white py-20">
      <div className="mx-auto max-w-7xl px-5 sm:px-6 lg:px-8">
        <div className="text-center">
          <p className="text-sm font-bold uppercase tracking-[0.22em] text-blue-600">
            Features that you need
          </p>

          <h2 className="mt-3 text-3xl font-black text-slate-950 sm:text-4xl lg:text-5xl">
            Everything Your Digital Card Needs
          </h2>

          <p className="mx-auto mt-4 max-w-2xl text-base leading-7 text-slate-500">
            Share your identity, contact information and business details using
            one professional digital card.
          </p>
        </div>

        <div className="mt-16 grid items-center gap-12 lg:grid-cols-[1fr_420px_1fr]">
          <div className="space-y-12">
            {leftFeatures.map((feature) => (
              <FeatureItem key={feature.title} {...feature} />
            ))}
          </div>

          <div className="mx-auto w-full max-w-[390px]">
            <div className="rounded-[3rem] bg-slate-950 p-3 shadow-2xl">
              <div className="relative overflow-hidden rounded-[2.45rem] bg-white">
                <div className="absolute left-1/2 top-0 h-7 w-36 -translate-x-1/2 rounded-b-3xl bg-slate-950" />

                <div className="bg-gradient-to-br from-blue-700 via-blue-600 to-cyan-500 px-7 pb-8 pt-14 text-white">
                  <div className="mx-auto flex h-24 w-24 items-center justify-center rounded-full border-4 border-white/80 bg-white text-3xl font-black text-blue-700 shadow-lg">
                    DC
                  </div>

                  <div className="mt-5 text-center">
                    <h3 className="text-2xl font-bold">Shahid</h3>
                    <p className="mt-1 text-sm text-blue-100">
                      Founder · Zenstrix
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-4 gap-3 border-b px-5 py-5">
                  {[
                    { icon: Phone, label: "Call" },
                    { icon: MessageCircle, label: "WhatsApp" },
                    { icon: Mail, label: "Email" },
                    { icon: Share2, label: "Share" },
                  ].map(({ icon: Icon, label }) => (
                    <div
                      key={label}
                      className="flex flex-col items-center gap-2 text-xs font-medium text-slate-600"
                    >
                      <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-blue-50 text-blue-600">
                        <Icon className="h-5 w-5" />
                      </span>
                      {label}
                    </div>
                  ))}
                </div>

                <div className="space-y-4 p-6">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-400">
                      Phone
                    </p>
                    <p className="mt-1 font-medium text-slate-800">
                      +91 98765 43210
                    </p>
                  </div>

                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-400">
                      Email
                    </p>
                    <p className="mt-1 font-medium text-slate-800">
                      hello@example.com
                    </p>
                  </div>

                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-slate-400">
                      Website
                    </p>
                    <p className="mt-1 font-medium text-blue-600">
                      www.example.com
                    </p>
                  </div>

                  <a href="/register" className="mt-2 block w-full rounded-xl bg-blue-600 px-4 py-3 text-center font-semibold text-white">
                    Create Your Card
                  </a>
                </div>
              </div>
            </div>
          </div>

          <div className="space-y-12">
            {rightFeatures.map((feature) => (
              <FeatureItem
                key={feature.title}
                {...feature}
                reverse
              />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
