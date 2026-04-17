"use client";

import React from "react";
import Image from "next/image";
import { FileImage, FileVideo, FileAudio, FileText } from "lucide-react";
import {
  getRawFileUrl,
  isImageFile,
  isPdfFile,
  isVideoFile,
  isAudioFile,
} from "@/lib/api";

interface BinaryFileViewerProps {
  repoId: number;
  path: string;
  refName?: string;
}

export function BinaryFileViewer({
  repoId,
  path,
  refName,
}: BinaryFileViewerProps) {
  const url = getRawFileUrl(repoId, path, refName);
  const fileName = path.split("/").pop() || "file";

  if (isImageFile(path)) {
    return (
      <div className="flex h-full w-full flex-col items-center gap-4">
        <FileImage className="h-16 w-16 flex-shrink-0 text-gray-400" />
        <div className="relative min-h-[300px] w-full flex-1">
          <Image
            src={url}
            alt={fileName}
            fill
            sizes="100vw"
            className="object-contain"
            unoptimized={true}
          />
        </div>
      </div>
    );
  }

  if (isPdfFile(path)) {
    return (
      <div className="flex h-full w-full flex-col items-center gap-4">
        <FileText className="h-16 w-16 text-gray-400" />
        <iframe
          src={url}
          className="h-full w-full border-0"
          title={`PDF preview of ${fileName}`}
        />
      </div>
    );
  }

  if (isVideoFile(path)) {
    return (
      <div className="flex flex-col items-center gap-4">
        <FileVideo className="h-16 w-16 text-gray-400" />
        <video src={url} controls className="max-h-full max-w-full">
          Your browser does not support the video tag.
        </video>
      </div>
    );
  }

  if (isAudioFile(path)) {
    return (
      <div className="flex flex-col items-center gap-4">
        <FileAudio className="h-16 w-16 text-gray-400" />
        <audio src={url} controls className="w-full max-w-md">
          Your browser does not support the audio tag.
        </audio>
      </div>
    );
  }

  return (
    <div className="text-center text-gray-500 dark:text-gray-400">
      <FileText className="mx-auto mb-4 h-16 w-16 text-gray-300 dark:text-gray-600" />
      <p className="mb-4">Binary file cannot be displayed</p>
      <a
        href={url}
        download
        className="inline-flex items-center gap-2 rounded-md bg-blue-500 px-4 py-2 text-white transition-colors hover:bg-blue-600"
      >
        Download file
      </a>
    </div>
  );
}
