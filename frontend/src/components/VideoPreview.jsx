import React from "react";
import PropTypes from "prop-types";

function VideoPreview({ proxy }) {
  return (
    <div
      style={{ display: "flex", flexDirection: "column", alignItems: "center" }}
    >
      <video controls width="300">
        <source src={proxy} type="video/mp4" /> {/* Ensure src is set */}
        Your browser does not support the video tag.
      </video>
    </div>
  );
}

VideoPreview.propTypes = {
  proxy: PropTypes.string.isRequired,
};

export default VideoPreview;
