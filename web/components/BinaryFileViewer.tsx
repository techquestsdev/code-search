"use client";

import React from "react";
import Image from "next/image";
import { FileImage, FileVideo, FileAudio, FileText } from "lucide-react";
import { getRawFileUrl, isImageFile, isPdfFile, isVideoFile, isAudioFile } from "@/lib/api";

interface BinaryFileViewerProps {
  repoId: number;
  path: string;
  refName?: string;
}

export function BinaryFileViewer({ repoId, path, refName }: BinaryFileViewerProps) {
  const url = getRawFileUrl(repoId, path, refName);
  const fileName = path.split("/").pop() || "file";

  if (isImageFile(path)) {
    return (
      <div className="flex flex-col items-center gap-4 w-full h-full">
        <FileImage className="w-16 h-16 text-gray-400 flex-shrink-0" />
        <div className="relative w-full flex-1 min-h-[300px]">
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
      <div className="flex flex-col items-center gap-4 w-full h-full">
        <FileText className="w-16 h-16 text-gray-400" />
        <iframe
          src={url}
          className="w-full h-full border-0"
          title={`PDF preview of ${fileName}`}
        />
      </div>
    );
  }

  if (isVideoFile(path)) {
    return (
      <div className="flex flex-col items-center gap-4">
        <FileVideo className="w-16 h-16 text-gray-400" />
        <video src={url} controls className="max-w-full max-h-full">
          Your browser does not support the video tag.
        </video>
      </div>
    );
  }

  if (isAudioFile(path)) {
    return (
      <div className="flex flex-col items-center gap-4">
        <FileAudio className="w-16 h-16 text-gray-400" />
        <audio src={url} controls className="w-full max-w-md">
          Your browser does not support the audio tag.
        </audio>
      </div>
    );
  }

  return (
    <div className="text-center text-gray-500 dark:text-gray-400">
      <FileText className="w-16 h-16 mx-auto mb-4 text-gray-300 dark:text-gray-600" />
      <p className="mb-4">Binary file cannot be displayed</p>
      <a
        href={url}
        download
        className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600 transition-colors"
      >
        Download file
      </a>
    </div>
  );
}
