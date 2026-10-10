let loading: Promise<void> | null = null;

/** Load only when checkout is requested; concurrent clicks share one script. */
export function loadRazorpayCheckout(): Promise<void> {
  if (window.Razorpay) return Promise.resolve();
  if (loading) return loading;
  loading = new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.id = "razorpay-checkout";
    script.src = "https://checkout.razorpay.com/v1/checkout.js";
    script.async = true;
    const fail = () => {
      clearTimeout(timeout);
      script.remove();
      reject(new Error("Unable to load secure checkout. Please check your connection and try again."));
    };
    const timeout = setTimeout(fail, 20000);
    script.onload = () => {
      clearTimeout(timeout);
      if (window.Razorpay) resolve(); else fail();
    };
    script.onerror = fail;
    document.body.appendChild(script);
  }).catch(error => {
    loading = null;
    throw error;
  });
  return loading;
}
