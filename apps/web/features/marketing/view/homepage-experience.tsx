"use client";

import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import styles from "./homepage-experience.module.css";

const EXIT_TRANSITION_MS = 220;
const INITIAL_PROGRESS = 10;
const MAX_PENDING_PROGRESS = 96;

const HomepageReadyContext = createContext(true);

function calculateCompletionDuration(elapsedMs: number, remaining: number) {
  const observedLoadFactor = Math.sqrt(Math.max(elapsedMs, 1)) * 8;
  const remainingDistanceFactor = remaining * 5;
  return Math.round(
    Math.min(620, Math.max(240, observedLoadFactor + remainingDistanceFactor)),
  );
}

function waitForDocumentLoad(cleanups: Array<() => void>) {
  if (document.readyState === "complete") {
    return Promise.resolve();
  }

  return new Promise<void>((resolve) => {
    const onLoad = () => resolve();
    window.addEventListener("load", onLoad, { once: true });
    cleanups.push(() => window.removeEventListener("load", onLoad));
  });
}

function waitForFonts() {
  return document.fonts?.ready.then(() => undefined) ?? Promise.resolve();
}

function waitForCriticalImages(cleanups: Array<() => void>) {
  const pendingImages = Array.from(document.images).filter(
    (image) => !image.complete,
  );

  if (pendingImages.length === 0) {
    return Promise.resolve();
  }

  return Promise.all(
    pendingImages.map(
      (image) =>
        new Promise<void>((resolve) => {
          const finish = () => resolve();
          image.addEventListener("load", finish, { once: true });
          image.addEventListener("error", finish, { once: true });
          cleanups.push(() => {
            image.removeEventListener("load", finish);
            image.removeEventListener("error", finish);
          });
        }),
    ),
  ).then(() => undefined);
}

export function useHomepageReady() {
  return useContext(HomepageReadyContext);
}

export function HomepageExperience({ children }: { children: ReactNode }) {
  const [phase, setPhase] = useState<"loading" | "exiting" | "ready">(
    "loading",
  );
  const [progress, setProgress] = useState(INITIAL_PROGRESS);
  const [progressDuration, setProgressDuration] = useState(280);
  const progressRef = useRef(INITIAL_PROGRESS);

  useEffect(() => {
    const reducedMotion =
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    if (reducedMotion) {
      let active = true;
      Promise.resolve().then(() => {
        if (!active) return;
        progressRef.current = 100;
        setProgress(100);
        setPhase("ready");
      });
      return () => {
        active = false;
      };
    }

    let active = true;
    let completionTimer: ReturnType<typeof setTimeout> | undefined;
    let exitTimer: ReturnType<typeof setTimeout> | undefined;
    const cleanups: Array<() => void> = [];
    const startedAt = performance.now();

    const reportReady = (weight: number) => {
      if (!active) return;
      const nextProgress = Math.min(
        MAX_PENDING_PROGRESS,
        progressRef.current + weight,
      );
      progressRef.current = Math.max(progressRef.current, nextProgress);
      setProgress(progressRef.current);
    };

    const readinessTasks = [
      waitForDocumentLoad(cleanups).then(() => reportReady(40)),
      waitForFonts().then(() => reportReady(25)),
      waitForCriticalImages(cleanups).then(() => reportReady(25)),
    ];

    Promise.all(readinessTasks).then(() => {
      if (!active) return;

      const completionDuration = calculateCompletionDuration(
        performance.now() - startedAt,
        100 - progressRef.current,
      );
      setProgressDuration(completionDuration);
      progressRef.current = 100;
      setProgress(100);

      completionTimer = setTimeout(() => {
        if (!active) return;
        setPhase("exiting");
        exitTimer = setTimeout(() => {
          if (active) setPhase("ready");
        }, EXIT_TRANSITION_MS);
      }, completionDuration);
    });

    return () => {
      active = false;
      cleanups.forEach((cleanup) => cleanup());
      clearTimeout(completionTimer);
      clearTimeout(exitTimer);
    };
  }, []);

  const isReady = phase === "ready";
  const progressStyle = {
    "--agentpay-progress": progress / 100,
    "--agentpay-progress-duration": `${progressDuration}ms`,
  } as CSSProperties;

  return (
    <HomepageReadyContext.Provider value={isReady}>
      <div className={styles.shell}>
        {phase !== "ready" ? (
          <div
            id="agentpay-preloader"
            className={styles.preloader}
            data-phase={phase}
          >
            <div className={styles.preloaderInner}>
              <div className={styles.preloaderTopline}>
                <span className={styles.preloaderBrand}>AgentPay</span>
                <span className={styles.preloaderValue}>{progress}%</span>
              </div>
              <div
                className={styles.preloaderTrack}
                role="progressbar"
                aria-label="Preparing AgentPay"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={progress}
              >
                <div
                  className={styles.preloaderProgress}
                  style={progressStyle}
                />
              </div>
              <p className={styles.preloaderStatus} aria-live="polite">
                Preparing the storefront
              </p>
            </div>
          </div>
        ) : null}
        <noscript>
          <style>{"#agentpay-preloader{display:none}"}</style>
        </noscript>
        {children}
      </div>
    </HomepageReadyContext.Provider>
  );
}
